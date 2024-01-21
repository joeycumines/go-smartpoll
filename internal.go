package smartpoll

import (
	"fmt"
	"time"
)

type (
	// Internal models the internal API, accessible from within hooks.
	// It is unsafe to retain a reference to Internal, or to use concurrently.
	Internal struct {
		scheduler *Scheduler
	}
)

// Next returns an advisory timestamp for when the given task key will next be
// ready, or the zero time if the task is not scheduled.
func (x *Internal) Next(key any) time.Time {
	return x.scheduler.task(key).next
}

// Running returns true if the given task key is currently running.
func (x *Internal) Running(key any) bool {
	return x.scheduler.task(key).running.Load() != 0
}

// StopTimer stops the timer for the given task key, but does not clear any
// scheduling of that task. It returns true if the task is still scheduled.
// Note that a task may be running, regardless of whether it is scheduled.
func (x *Internal) StopTimer(key any) (ready bool) {
	return x.scheduler.task(key).
		stopTimer()
}

// Schedule schedules the given task key to be ready after the given duration,
// immediately if the duration is zero, or clears any scheduling if the
// duration is negative. The scheduled time will be returned by Next, until it
// is consumed, or the task is rescheduled.
func (x *Internal) Schedule(key any, d time.Duration) {
	x.scheduler.task(key).
		setTimer(d)
}

// ScheduleAt schedules the given task key to be ready after the given time,
// or clears any scheduling if t is the zero value.
// The return value of Next will be the provided time, until it is consumed,
// or the task is rescheduled.
func (x *Internal) ScheduleAt(key any, t time.Time) {
	x.scheduler.task(key).
		setTimerAt(t)
}

// ScheduleSooner is like Schedule, except it will use the sooner of the
// existing schedule and the given duration.
// It will panic if the given duration is not positive (use Schedule if you
// wish to immediately schedule a task).
func (x *Internal) ScheduleSooner(key any, d time.Duration) {
	if d <= 0 {
		panic(fmt.Sprintf(`smartpoll: cannot schedule sooner of invalid duration: %s`, d))
	}
	x.ScheduleAtSooner(key, time.Now().Add(d))
}

// ScheduleAtSooner is like ScheduleAt, except it will use the sooner of the
// existing schedule and the given time.
// It will panic if the given time is the zero value.
func (x *Internal) ScheduleAtSooner(key any, t time.Time) {
	x.scheduler.task(key).
		rescheduleSooner(t)
}
