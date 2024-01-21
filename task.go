package smartpoll

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type (
	// Task performs an arbitrary operation in the background, returning either
	// a fatal error, or a TaskHook.
	// The context provided to Task will be cancelled after this function (not
	// the TaskHook) returns, or when the Scheduler.Run context is canceled.
	//
	// Each configured task runs independently, and only one will run per key,
	// at any given time.
	Task func(ctx context.Context) (TaskHook, error)

	// TaskHook performs an arbitrary operation, synchronised with the main
	// loop, within Scheduler.Run. The typical usage of this is to handle
	// results (e.g. store them in an in-memory cache), and/or to reschedule
	// the task.
	// The context provided to TaskHook will be cancelled after this function
	// returns, or the Scheduler.Run context is canceled.
	//
	// Like all "hooks" in this package, TaskHook runs synchronously with the
	// main loop. It will always be run, prior to the next Task (of the same
	// key), though it may not be called, in the event a fatal error occurs.
	TaskHook func(ctx context.Context, internal *Internal) error

	taskState struct {
		task Task
		// timer has three possible states:
		//
		//   1. nil - task is not scheduled
		//   2. readySentinel - task is scheduled and ready to be executed
		//   3. time.Timer - task is scheduled and will be ready after the timer fires
		timer *time.Timer
		// next is an advisory timestamp for when the task will next be ready.
		// It is consumed alongside timer, when the value is received.
		next time.Time
		// running will be 0 if the task is not running, 1 if it is
		running atomic.Int32
	}
)

var (
	// ErrPanicInTask will be returned by Scheduler.Run if a task calls
	// runtime.Goexit. Note that it will technically also be returned if a task
	// panics, but that will bubble, and cause the process to exit.
	ErrPanicInTask = errors.New(`smartpoll: panic in task`)

	// identifies tasks which are ready to be executed
	readySentinel = new(time.Timer)
)

func (x Task) call(ctx context.Context) (TaskHook, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return x(ctx)
}

func (x TaskHook) call(ctx context.Context, internal *Internal) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return x(ctx, internal)
}

func (x *taskState) run(ctx context.Context, scheduler *Scheduler, internal *Internal) {
	var success bool
	var clearedRunning bool
	clearRunning := func() {
		if !clearedRunning {
			clearedRunning = true
			// mark the task as no longer running
			x.running.Store(0)
			// notify, for the edge case of re-running the scheduler
			select {
			case scheduler.taskCompleteCh <- struct{}{}:
			default:
			}
		}
	}
	defer func() {
		if !success {
			// for the purposes of treating runtime.Goexit as a fatal error
			select {
			case <-ctx.Done():
			case scheduler.taskLockCh <- ErrPanicInTask:
			}
		}
		clearRunning()
	}()

	// run the (background) part of the task
	hook, err := x.task.call(ctx)

	// block the main loop / notify it of any error
	select {
	case <-ctx.Done():
		success = true
		return
	case scheduler.taskLockCh <- err:
	}

	// if we sent an error, it will result in a fatal error (we don't need to unlock)
	if err != nil {
		success = true
		return
	}

	defer func() {
		// BEFORE we continue, while the main loop is blocked, we must allow
		// the task to be rerun. If we tried to do this after, there's a small
		// chance it'll race, and fail to "wake up" the main loop, which is
		// responsible for re-running the task (in the case where another run
		// has been scheduled, prior to the task finishing).
		clearRunning()

		// same deal as the defer above (handle runtime.Goexit as a fatal error)
		if !success {
			success = true // we only need to send one fatal error
			err = ErrPanicInTask
		}

		// once we are done, unblock the main loop / notify it of error
		// WARNING: This can't select on context, or it'll may cause a data
		// race (e.g. in the deferred timer stops, in the main loop).
		scheduler.taskUnlockCh <- err
	}()

	// finally, we run the hook (err is handled in the defer above)
	err = hook.call(ctx, internal)
	success = true
}

func (x *taskState) startIfPossible(ctx context.Context, scheduler *Scheduler, internal *Internal) {
	if x.timer != readySentinel || !x.running.CompareAndSwap(0, 1) {
		// not scheduled to start, or still running
		return
	}

	// consume our ready tick
	x.timer = nil
	x.next = time.Time{}

	// run the task in the background (it will synchronise using Scheduler.taskLockCh / taskSyncInternalCaseIndex)
	go x.run(ctx, scheduler, internal)
}

// setTimer forcibly updates the state of the timer, disabling it if negative,
// setting it to readySentinel if zero, or setting it to a new timer if
// positive.
// It also updates next, the advisory timestamp for when the task will next be
// ready.
func (x *taskState) setTimer(d time.Duration) {
	if x.timer != nil && x.timer != readySentinel {
		// stop and drain (we consume any ready tick)
		if !x.timer.Stop() {
			select {
			case <-x.timer.C:
			default:
			}
		}
	}

	// we update both next and timer, unless both the current and desired state is ready
	switch {
	case d > 0:
		// take now before starting the timer (preserve "at least d" semantics)
		x.next = time.Now().Add(d)
		x.timer = time.NewTimer(d)

	case d < 0:
		x.next = time.Time{}
		x.timer = nil

	case x.timer != readySentinel:
		x.next = time.Now()
		x.timer = readySentinel
	}
}

func (x *taskState) setTimerAt(t time.Time) {
	if t == (time.Time{}) {
		x.setTimer(-1)
		return
	}
	stopTimer(&x.timer)
	x.next = t
	x.timer = startTimer(t.Sub(time.Now()))
}

func (x *taskState) rescheduleSooner(t time.Time) {
	if t == (time.Time{}) {
		panic(`smartpoll: cannot schedule at sooner of zero time`)
	}

	if x.timer != nil && x.next != (time.Time{}) && !x.next.After(t) {
		// already scheduled sooner / at the same time
		return
	}

	x.next = t

	if stopTimer(&x.timer) {
		// keep the existing readiness - next was after t
		return
	}

	x.timer = startTimer(t.Sub(time.Now()))
}

func (x *taskState) stopTimer() (ready bool) {
	ready = stopTimer(&x.timer)
	if !ready {
		x.next = time.Time{}
	}
	return
}

// stopTimer will ensure the timer is stopped, resolving it to either nil or
// readySentinel, returning a boolean to indicate which.
// The underlying time.Timer will be stopped (if any), and its channel drained.
//
// WARNING: Use taskState.stopTimer, in order to correctly update next.
func stopTimer(p **time.Timer) (ready bool) {
	v := *p

	switch v {
	case nil:
		return false
	case readySentinel:
		return true
	}

	if !v.Stop() {
		select {
		case <-v.C:
			ready = true
		default:
		}
	}

	if ready {
		*p = readySentinel
	} else {
		*p = nil
	}

	return ready
}

// startTimer initializes the timer, and returns it, using readySentinel for
// negative or zero durations.
func startTimer(d time.Duration) *time.Timer {
	if d > 0 {
		return time.NewTimer(d)
	}
	return readySentinel
}
