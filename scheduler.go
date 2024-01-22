package smartpoll

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"
)

const (
	// maxSelectCases is the maximum number of cases supported by reflect.Select
	maxSelectCases = 65536

	// contextInternalCaseIndex is the index of the select case used for context cancel
	contextInternalCaseIndex = iota - 1
	// taskSyncInternalCaseIndex is the index of the select case used for task synchronization
	taskSyncInternalCaseIndex
	// numInternalCases is the total number of internal cases, which sit between the select cases for hooks and tasks
	numInternalCases
)

type (
	// Scheduler schedules "tasks", implements a "main loop", and provides
	// various means to manager the scheduler (and other, arbitrary) state, via
	// "hooks", which run within, or are synchronised with, the main loop.
	//
	// Scheduler must be constructed with New. The Run method is used to run
	// the scheduler.
	//
	// See also the package docs for [smartpoll].
	Scheduler struct {
		// tasks are all configured Task values + schedule state, identified by an arbitrary key
		tasks map[any]*taskState

		// taskLockCh receives responses from the first stage of a Task.
		// If the value is nil, the main loop blocks on the taskUnlockCh.
		taskLockCh chan error

		// taskUnlockCh receives responses from the second stage of a Task.
		taskUnlockCh chan error

		// taskCompleteCh is buffered, and is used to wake up a Scheduler.Run
		// call which is blocking on tasks from a previous call.
		taskCompleteCh chan struct{}

		// hooks are the Hook.call methods for each custom hook
		hooks []func(ctx context.Context, internal *Internal, value reflect.Value, ok bool) error

		// cases are hooks + internal cases + task cases, see also initCases, numInternalCases
		cases []reflect.SelectCase

		// taskIndices maps the (relative) index of a task case to the task key
		taskIndices []any

		// runHooks are called on each Scheduler.Run, just prior to starting the main loop.
		runHooks []RunHook

		// running is used to trigger a panic if Run is called concurrently
		running atomic.Int32
	}
)

// Run runs the scheduler, blocking until the context is cancelled, or an error
// is returned from a Hook or a Task. A panic will occur if called
// concurrently (called again before the previous call returns), or if called
// on a scheduler which was not initialized with New.
//
// The context, passed to tasks and hooks, will be derived from the context
// passed to Run, and will be cancelled when Run returns. There is not any
// built-in mechanism to cancel individual running tasks.
//
// It is important to note that Run does not wait for all running tasks to
// complete, prior to exit. Calling Run again will block on any tasks which are
// still running, dropping any errors (likely context cancellation) which occur.
func (x *Scheduler) Run(ctx context.Context) error {
	if len(x.tasks) == 0 {
		panic(`smartpoll: scheduler must be initialized with New`)
	}

	// prevent more than one run call at a time (the behavior would be very poor)
	if !x.running.CompareAndSwap(0, 1) {
		panic(`smartpoll: scheduler already running`)
	}
	defer x.running.Store(0)

	// we want to cancel task context on exit
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// wait for any tasks from a previous run to complete
	for x.hasRunningTasks() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-x.taskCompleteCh:
		}
	}

	// always stop all timers, and clear all ready tasks, on exit
	defer x.unscheduleAllTasks()

	// init/update contextInternalCaseIndex
	x.setContext(ctx)

	// the internal scheduler api, for hooks, and the synchronised parts of tasks
	internal := Internal{scheduler: x}

	// run any configured run hooks
	for _, hook := range x.runHooks {
		if err := hook.call(ctx, &internal); err != nil {
			return err
		}
	}

	// scheduler main loop
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// start tasks which are scheduled and are not running
		for _, state := range x.tasks {
			state.startIfPossible(ctx, x, &internal)
		}

		// init/update the task cases
		x.updateTaskCases()

		// wait for our next thing to do
		i, v, ok := reflect.Select(x.cases)
		switch {
		case i < x.absInternalCaseIndex(0):
			// hook case - we've received a value, which we pass to the appropriate hook
			if err := x.hooks[i](ctx, &internal, v, ok); err != nil {
				return err
			}

		case i >= x.absTaskCaseIndex(0):
			// task case - the given task is now ready
			x.task(x.taskIndices[i-x.absTaskCaseIndex(0)]).timer = readySentinel

		case i == x.absInternalCaseIndex(contextInternalCaseIndex):
			return ctx.Err()

		case i == x.absInternalCaseIndex(taskSyncInternalCaseIndex):
			// can't assert `error` yet (might be nil interface{})
			err := v.Interface()
			switch err {
			case nil:
				// WARNING: This can't select on context, or it may cause a
				// data race (e.g. in the deferred timer stops).
				if err := <-x.taskUnlockCh; err != nil {
					return err
				}

			case noHookSentinel:
				// nothing to do - just used to skip unlock

			default:
				// type assertion must be after the nil check
				return err.(error)
			}

		default:
			panic(fmt.Sprintf(`smartpoll: unexpected select case index: %d`, i))
		}
	}
}

// task retrieves the task state, and will panic if the key is not found.
func (x *Scheduler) task(key any) (state *taskState) {
	state = x.tasks[key]
	if state == nil {
		panic(fmt.Sprintf(`smartpoll: task not found: %T: %v`, key, key))
	}
	return
}

// initCases runs after cases for hooks have been assigned.
// See also setContext and updateTaskCases, both of which are necessary to
// finish preparing the cases for reflect.Select.
func (x *Scheduler) initCases() error {
	// should be one case per (custom) hook
	if len(x.hooks) != len(x.cases) {
		panic(`smartpoll: init internal cases unexpected mismatch of hooks and cases`)
	}

	if requiredLen := len(x.hooks) + numInternalCases + len(x.tasks); requiredLen > maxSelectCases {
		return fmt.Errorf(`smartpoll: init internal cases too many cases: %d > %d`, requiredLen, maxSelectCases)
	} else {
		cases := make([]reflect.SelectCase, requiredLen)
		copy(cases, x.cases)
		x.cases = cases
	}

	x.cases[x.absInternalCaseIndex(taskSyncInternalCaseIndex)] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(x.taskLockCh),
	}

	// assign an index to each task
	x.taskIndices = make([]any, 0, len(x.tasks))
	for key := range x.tasks {
		x.taskIndices = append(x.taskIndices, key)
	}

	return nil
}

func (x *Scheduler) absInternalCaseIndex(i int) int {
	return len(x.hooks) + i
}

func (x *Scheduler) absTaskCaseIndex(i int) int {
	return len(x.hooks) + numInternalCases + i
}

// setContext updates the (internal) select case used for context cancel.
func (x *Scheduler) setContext(ctx context.Context) {
	x.cases[x.absInternalCaseIndex(contextInternalCaseIndex)] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}
}

// updateTaskCases assigns the appropriate channel to each task case.
func (x *Scheduler) updateTaskCases() {
	for i, key := range x.taskIndices {
		var ch <-chan time.Time
		if state := x.task(key); state.timer != nil {
			ch = state.timer.C // note: will be nil if readySentinel
		}
		x.cases[x.absTaskCaseIndex(i)] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
	}
}

func (x *Scheduler) hasRunningTasks() bool {
	for _, state := range x.tasks {
		if state.running.Load() != 0 {
			return true
		}
	}
	return false
}

func (x *Scheduler) unscheduleAllTasks() {
	for _, state := range x.tasks {
		state.setTimer(-1)
	}
}
