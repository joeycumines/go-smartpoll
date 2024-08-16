package smartpoll

import (
	"testing"
	"time"
)

func TestInternal_Next(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {
				next: time.Now().Add(time.Minute),
			},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Call the Next method
	next := internal.Next("task1")

	// Verify that the returned time is the same as the one we set in the taskState
	if next != scheduler.tasks["task1"].next {
		t.Errorf("Expected %v, got %v", scheduler.tasks["task1"].next, next)
	}
}

func TestInternal_Running(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Set the task as running
	scheduler.tasks["task1"].running.Store(1)

	// Call the Running method
	running := internal.Running("task1")

	// Verify that the returned value is true
	if !running {
		t.Errorf("Expected task to be running")
	}

	// Set the task as not running
	scheduler.tasks["task1"].running.Store(0)

	// Call the Running method again
	running = internal.Running("task1")

	// Verify that the returned value is false
	if running {
		t.Errorf("Expected task to not be running")
	}
}

func TestInternal_StopTimer_timerDrained(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {
				timer: startTimer(1),
			},
		},
	}

	time.Sleep(time.Millisecond * 100)

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Call the StopTimer method
	ready := internal.StopTimer("task1")

	// Verify that the returned value is true
	if !ready {
		t.Errorf("Expected task to be ready")
	}

	if scheduler.tasks["task1"].timer != readySentinel {
		t.Errorf("Expected task timer to be nil")
	}
}

func TestInternal_Schedule(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Call the Schedule method
	internal.Schedule("task1", time.Minute)

	// Verify that the timer for the task is not nil
	if scheduler.tasks["task1"].timer == nil {
		t.Errorf("Expected task timer to not be nil")
	}

	// Verify that the next time for the task is not zero
	if scheduler.tasks["task1"].next == (time.Time{}) {
		t.Errorf("Expected task next time to not be zero")
	}
}

func TestInternal_ScheduleAt(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Define a time for scheduling
	scheduleTime := time.Now().Add(time.Minute)

	// Call the ScheduleAt method
	internal.ScheduleAt("task1", scheduleTime)

	// Verify that the timer for the task is not nil
	if scheduler.tasks["task1"].timer == nil {
		t.Errorf("Expected task timer to not be nil")
	}

	// Verify that the next time for the task is the same as the scheduled time
	if scheduler.tasks["task1"].next != scheduleTime {
		t.Errorf("Expected task next time to be %v, got %v", scheduleTime, scheduler.tasks["task1"].next)
	}
}

func TestInternal_ScheduleSooner(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Call the Schedule method with a longer duration
	internal.Schedule("task1", time.Minute*2)

	// Call the ScheduleSooner method with a shorter duration
	internal.ScheduleSooner("task1", time.Minute)

	// Verify that the timer for the task is not nil
	if scheduler.tasks["task1"].timer == nil {
		t.Errorf("Expected task timer to not be nil")
	}

	// Verify that the next time for the task is sooner than the original schedule
	if scheduler.tasks["task1"].next.After(time.Now().Add(time.Minute * 2)) {
		t.Errorf("Expected task next time to be sooner")
	}
}

func TestInternal_StopTimer_timerNotDrained(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {
				timer: startTimer(time.Minute),
			},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	ready := internal.StopTimer("task1")
	if ready {
		t.Errorf("Expected task not to be ready")
	}

	if scheduler.tasks["task1"].timer != nil {
		t.Errorf("Expected task timer to be nil")
	}
}

func TestInternal_ScheduleAtSooner(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Define a time for scheduling
	laterTime := time.Now().Add(time.Minute * 2)

	// Call the ScheduleAt method with a later time
	internal.ScheduleAt("task1", laterTime)

	// Define a sooner time for scheduling
	soonerTime := time.Now().Add(time.Minute)

	// Call the ScheduleAtSooner method with a sooner time
	internal.ScheduleAtSooner("task1", soonerTime)

	// Verify that the timer for the task is not nil
	if scheduler.tasks["task1"].timer == nil {
		t.Errorf("Expected task timer to not be nil")
	}

	// Verify that the next time for the task is the same as the sooner time
	if scheduler.tasks["task1"].next != soonerTime {
		t.Errorf("Expected task next time to be %v, got %v", soonerTime, scheduler.tasks["task1"].next)
	}

	// Define a later time for scheduling
	laterTime = time.Now().Add(time.Minute * 3)

	// Call the ScheduleAtSooner method with a later time
	internal.ScheduleAtSooner("task1", laterTime)

	// Verify that the next time for the task is still the same as the sooner time
	if scheduler.tasks["task1"].next != soonerTime {
		t.Errorf("Expected task next time to still be %v, got %v", soonerTime, scheduler.tasks["task1"].next)
	}

	// verify that scheduling with the same time does nothing
	existingTimer := scheduler.tasks["task1"].timer
	internal.ScheduleAtSooner("task1", soonerTime)
	if scheduler.tasks["task1"].timer != existingTimer {
		t.Fatal()
	}
	internal.ScheduleAtSooner("task1", soonerTime.UTC())
	if scheduler.tasks["task1"].timer != existingTimer {
		t.Fatal()
	}
	if scheduler.tasks["task1"].next != soonerTime {
		t.Fatal()
	}

	// reschedule sooner, and verify that the old timer was stopped
	internal.ScheduleAtSooner("task1", soonerTime.Add(-1))
	if scheduler.tasks["task1"].timer == existingTimer || scheduler.tasks["task1"].timer == nil {
		t.Fatal()
	}
	select {
	case <-existingTimer.C:
		t.Fatal()
	default:
	}
	if existingTimer.Stop() {
		t.Fatal()
	}
	if !scheduler.tasks["task1"].next.Equal(soonerTime.Add(-1)) {
		t.Fatal()
	}
}

func TestInternal_ScheduleSooner_InvalidDuration(t *testing.T) {
	// Initialize a new Scheduler
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}

	// Initialize a new Internal
	internal := &Internal{
		scheduler: scheduler,
	}

	// Define a non-positive duration
	invalidDuration := time.Duration(0)

	// Call the ScheduleSooner method with the invalid duration and expect a panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	internal.ScheduleSooner("task1", invalidDuration)
}

func TestInternal_ScheduleAtSooner_zeroValue(t *testing.T) {
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {},
		},
	}
	internal := &Internal{
		scheduler: scheduler,
	}
	defer func() {
		if r := recover(); r != `smartpoll: cannot schedule at sooner of zero time` {
			t.Error(r)
		}
	}()
	internal.ScheduleAtSooner("task1", time.Time{})
}

func TestInternal_ScheduleAtSooner_alreadyReady(t *testing.T) {
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {
				timer: readySentinel,
			},
		},
	}
	internal := &Internal{
		scheduler: scheduler,
	}
	internal.ScheduleAtSooner("task1", time.Unix(0, 0))
	if v := scheduler.tasks["task1"].timer; v != readySentinel {
		t.Error(v)
	}
	if v := scheduler.tasks["task1"].next; !v.Equal(time.Unix(0, 0)) {
		t.Error(v)
	}
}

func TestInternal_ScheduleAtSooner_alreadyReadyNotSentinel(t *testing.T) {
	lower := time.Now().Add(time.Minute)
	upper := lower.Add(1)
	timer := time.NewTimer(1)
	time.Sleep(time.Millisecond * 30)
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {
				next:  upper,
				timer: timer,
			},
		},
	}
	internal := &Internal{
		scheduler: scheduler,
	}
	internal.ScheduleAtSooner("task1", lower)
	// N.B. will get swapped out to be readySentinel while checking if ready
	if v := scheduler.tasks["task1"].timer; v != readySentinel {
		t.Error(v)
	}
	if v := scheduler.tasks["task1"].next; !v.Equal(lower) || v.Equal(upper) {
		t.Error(v)
	}
}

func TestInternal_ScheduleAt_clearIfZeroValue(t *testing.T) {
	scheduler := &Scheduler{
		tasks: map[any]*taskState{
			"task1": {
				next:  time.Now(),
				timer: readySentinel,
			},
		},
	}
	internal := &Internal{
		scheduler: scheduler,
	}
	internal.ScheduleAt("task1", time.Time{})
	if v := scheduler.tasks["task1"].timer; v != nil {
		t.Error(v)
	}
	if v := scheduler.tasks["task1"].next; v != (time.Time{}) {
		t.Error(v)
	}
}
