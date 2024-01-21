package smartpoll

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

// Tests that tasks are able to run independently, kicking them off using a
// RunHook. After an arbitrary number of times (different per task), with some
// very short arbitrary sleeps, no more runs are performed and
// sync.WaitGroup.Done is called. After all tasks have completed, the context
// is cancelled, and the main loop exits. The number of runs for each task is
// verified.
func TestScheduler_Run_runIndependentTasks(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	taskRuns := map[string]int{
		"task1": 5,
		"task2": 3,
		"task3": 7,
	}
	runCounts := make(map[string]int)
	var wg sync.WaitGroup
	wg.Add(len(taskRuns))

	options := []Option{
		WithRunHook(func(ctx context.Context, internal *Internal) error {
			for k := range taskRuns {
				internal.Schedule(k, 0)
			}
			return nil
		}),
	}

	for taskName, numRuns := range taskRuns {
		taskName := taskName
		numRuns := numRuns
		options = append(options, WithTask(taskName, func(ctx context.Context) (TaskHook, error) {
			time.Sleep(time.Millisecond * 3)
			return func(ctx context.Context, internal *Internal) error {
				runCounts[taskName]++
				if runCounts[taskName] < numRuns {
					internal.Schedule(taskName, time.Millisecond)
				} else {
					wg.Done()
				}
				return nil
			}, nil
		}))
	}

	// Create the scheduler with the tasks
	scheduler, err := New(options...)
	if err != nil {
		t.Fatalf("Failed to create scheduler: %v", err)
	}

	// Run the scheduler in a separate goroutine
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	out := make(chan error)
	go func() {
		out <- scheduler.Run(ctx)
	}()

	wg.Wait()

	cancel()

	if err := <-out; err != context.Canceled {
		t.Errorf("Scheduler returned an error: %v", err)
	}

	// Check that each task has run the expected number of times
	for taskName, numRuns := range taskRuns {
		if runCounts[taskName] != numRuns {
			t.Errorf("Task %s ran %d times, expected %d", taskName, runCounts[taskName], numRuns)
		}
	}
}

// Based on TestScheduler_Run_runIndependentTasks, this test throws in a
// single hook, which kicks off the tasks. It also verifies the sanity of all
// the Scheduler.cases.
func TestScheduler_Run_runWithHook(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	taskRuns := map[string]int{
		"task1": 5,
		"task2": 3,
		"task3": 7,
	}
	runCounts := make(map[string]*int32, len(taskRuns))
	for k := range taskRuns {
		runCounts[k] = new(int32)
	}
	var wg sync.WaitGroup
	wg.Add(len(taskRuns))

	startAllTasksCh := make(chan string)
	var startedAndAllTasksRunning sync.WaitGroup
	startedAndAllTasksRunning.Add(len(taskRuns) + 1)
	startedAndAllTasksRunningCh := make(chan struct{})
	go func() {
		startedAndAllTasksRunning.Wait()
		close(startedAndAllTasksRunningCh)
	}()

	const expectedStartAllTasksValue = "startAllTasks"
	var scheduler *Scheduler
	options := []Option{
		WithHook(startAllTasksCh, func(ctx context.Context, internal *Internal, value string, ok bool) error {
			if internal.scheduler != scheduler {
				t.Error(`unexpected scheduler`)
			}
			for _, v := range scheduler.cases {
				if v.Dir != reflect.SelectRecv {
					t.Error(`unexpected case direction`, v.Dir)
				}
				if !v.Chan.IsValid() || v.Chan.Kind() != reflect.Chan {
					t.Error(`unexpected case channel`, v.Chan)
				}
			}
			if value != expectedStartAllTasksValue {
				t.Errorf("Received unexpected value %v, expected %v", value, expectedStartAllTasksValue)
			}
			if !ok {
				t.Errorf("Received unexpected closed channel")
			}
			for k := range taskRuns {
				internal.Schedule(k, 0)
			}
			startedAndAllTasksRunning.Done()
			return nil
		}),
	}

	for taskName, numRuns := range taskRuns {
		taskName := taskName
		numRuns := numRuns
		options = append(options, WithTask(taskName, func(ctx context.Context) (TaskHook, error) {
			if *(runCounts[taskName]) == 0 {
				startedAndAllTasksRunning.Done()
				<-startedAndAllTasksRunningCh
			}
			time.Sleep(time.Millisecond * 3)
			*(runCounts[taskName]) = (*(runCounts[taskName])) + 1
			return func(ctx context.Context, internal *Internal) error {
				if int(*(runCounts[taskName])) < numRuns {
					internal.Schedule(taskName, time.Millisecond)
				} else {
					wg.Done()
				}
				return nil
			}, nil
		}))
	}

	// Create the scheduler with the tasks
	scheduler, err := New(options...)
	if err != nil {
		t.Fatalf("Failed to create scheduler: %v", err)
	}

	// Run the scheduler in a separate goroutine
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	out := make(chan error)
	go func() {
		out <- scheduler.Run(ctx)
	}()

	// wait for a bit - it shouldn't do anything, for the moment
	time.Sleep(time.Millisecond * 30)
	select {
	case <-out:
		t.Fatal(`Scheduler exited early`)
	default:
	}
	for taskName, runCount := range runCounts {
		if *runCount != 0 {
			t.Errorf("Task %s ran %d times, expected 0", taskName, *runCount)
		}
	}

	// start the tasks
	startAllTasksCh <- expectedStartAllTasksValue

	wg.Wait()

	cancel()

	if err := <-out; err != context.Canceled {
		t.Errorf("Scheduler returned an error: %v", err)
	}

	// Check that each task has run the expected number of times
	for taskName, numRuns := range taskRuns {
		if int(*(runCounts[taskName])) != numRuns {
			t.Errorf("Task %s ran %d times, expected %d", taskName, int(*(runCounts[taskName])), numRuns)
		}
	}
}

func TestScheduler_Run_multipleRuns(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	// start a scheduler with a single task, which will block until we tell it otherwise
	taskRunning := make(chan struct{})
	unblockTask := make(chan struct{})
	ee := errors.New(`some error`)
	sch, err := New(
		WithTask(nil, func(ctx context.Context) (TaskHook, error) {
			if err := ctx.Err(); err != nil {
				t.Error(err)
			}
			taskRunning <- struct{}{}
			<-unblockTask
			return nil, ee
		}),
		WithRunHook(func(ctx context.Context, internal *Internal) error {
			internal.Schedule(nil, 0)
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	out := make(chan error)
	run := func() context.CancelFunc {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			out <- sch.Run(ctx)
		}()
		return cancel
	}

	// starts immediately, exits with the task still running
	firstRun := func() {
		cancel := run()
		<-taskRunning
		cancel()
		if err := <-out; err != context.Canceled {
			t.Fatal(err)
		}
	}
	// blocks until we unblock the task, then exits due to task error
	secondRun := func() {
		cancel := run()
		defer cancel()

		time.Sleep(time.Millisecond * 30)

		select {
		case <-taskRunning:
			t.Fatal(`task should not be running`)
		case <-out:
			t.Fatal(`scheduler should not have exited`)
		default:
		}

		unblockTask <- struct{}{}

		<-taskRunning
		unblockTask <- struct{}{}

		if err := <-out; err != ee {
			t.Fatal(err)
		}
	}

	for i := 0; i < 3; i++ {
		firstRun()
		secondRun()
	}

	// finally, exit due to context cancellation, while blocking on the prior task

	firstRun()

	cancel := run()
	defer cancel()

	time.Sleep(time.Millisecond * 30)

	select {
	case <-taskRunning:
		t.Fatal(`task should not be running`)
	case <-out:
		t.Fatal(`scheduler should not have exited`)
	default:
	}

	cancel()

	if err := <-out; err != context.Canceled {
		t.Fatal(err)
	}

	secondRun()
}

func TestScheduler_Run_taskHookError(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	ee := errors.New(`some error`)
	var ctx1, ctx2 context.Context
	sch, err := New(
		WithTask(nil, func(ctx context.Context) (TaskHook, error) {
			ctx1 = ctx
			if err := ctx.Err(); err != nil {
				t.Error(err)
			}
			return func(ctx context.Context, internal *Internal) error {
				ctx2 = ctx
				if err := ctx.Err(); err != nil {
					t.Error(err)
				}
				return ee
			}, nil
		}),
		WithRunHook(func(ctx context.Context, internal *Internal) error {
			internal.ScheduleSooner(nil, time.Millisecond*10)
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := sch.Run(context.Background()); err != ee {
		t.Error(err)
	}

	if err := ctx1.Err(); err != context.Canceled {
		t.Error(err)
	}
	if err := ctx2.Err(); err != context.Canceled {
		t.Error(err)
	}
}

func TestScheduler_Run_runHookErrorAndClearingTimers(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	var task Task = func(ctx context.Context) (TaskHook, error) {
		const msg = `should not have been called`
		t.Error(msg)
		panic(msg)
	}
	ee := errors.New(`some error`)
	p1 := new(float64)
	var sch *Scheduler
	sch, err := New(
		WithTask(nil, task),
		WithTask(1, task),
		WithTask(true, task),
		WithTask(p1, task),
		WithRunHook(func(ctx context.Context, internal *Internal) error {
			now := time.Now()
			t1 := now.Add(time.Hour)
			t2 := now.Add(time.Hour * 2)
			internal.Schedule(1, time.Hour*6)
			internal.Schedule(1, time.Hour*5)
			internal.ScheduleAtSooner(1, t1)
			internal.ScheduleAt(p1, t2)
			internal.Schedule(true, time.Hour*4)
			if v := sch.tasks[nil]; v == nil || v.timer != nil || v.next != (time.Time{}) {
				t.Error()
			}
			if v := sch.tasks[1]; v == nil || v.timer == nil || v.next != t1 {
				t.Error()
			}
			if v := sch.tasks[p1]; v == nil || v.timer == nil || v.next != t2 {
				t.Error()
			}
			if v := sch.tasks[true]; v == nil || v.timer == nil {
				t.Error()
			}
			return ee
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Run(context.Background()); err != ee {
		t.Error(err)
	}
	if len(sch.tasks) != 4 {
		t.Error(len(sch.tasks))
	}
	for k, v := range sch.tasks {
		if v == nil || v.timer != nil || v.next != (time.Time{}) {
			t.Error(k)
		}
	}
}

func TestScheduler_Run_multipleHooksHookError(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	aIn := make(chan int)
	aOut := make(chan int)
	bIn := make(chan error)
	bOut := make(chan error)
	cIn := make(chan *int)
	cOut := make(chan *int)
	dIn := make(chan []string)
	dOut := make(chan []string)

	ee := errors.New(`some error`)

	sch, err := New(
		WithHook(aIn, func(ctx context.Context, internal *Internal, value int, ok bool) error {
			if !ok {
				t.Error(`unexpected closed channel`)
			}
			aOut <- value
			return nil
		}),
		WithHook(bIn, func(ctx context.Context, internal *Internal, value error, ok bool) error {
			if !ok {
				t.Error(`unexpected closed channel`)
			}
			bOut <- value
			return nil
		}),
		WithHook(cIn, func(ctx context.Context, internal *Internal, value *int, ok bool) error {
			if !ok {
				return ee
			}
			cOut <- value
			return nil
		}),
		WithHook(dIn, func(ctx context.Context, internal *Internal, value []string, ok bool) error {
			if !ok {
				t.Error(`unexpected closed channel`)
			}
			dOut <- value
			return nil
		}),
		// dummy task to avoid error on New
		WithTask(nil, func(ctx context.Context) (TaskHook, error) {
			const msg = `should not have been called`
			t.Error(msg)
			panic(msg)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	out := make(chan error)
	go func() {
		out <- sch.Run(context.Background())
	}()

	aIn <- 9123
	if v := <-aOut; v != 9123 {
		t.Error(v)
	}

	{
		e := errors.New(`asdasads`)
		bIn <- e
		if v := <-bOut; v != e {
			t.Error(v)
		}
	}

	{
		n := 123
		cIn <- &n
		if v := <-cOut; v != &n {
			t.Error(v)
		}
	}

	{
		dIn <- []string{`a`, `b`, `c`}
		if v := <-dOut; len(v) != 3 || v[0] != `a` || v[1] != `b` || v[2] != `c` {
			t.Error(v)
		}
	}

	bIn <- nil
	if v := <-bOut; v != nil {
		t.Error(v)
	}

	{
		e := errors.New(`z`)
		bIn <- e
		if v := <-bOut; v != e {
			t.Error(v)
		}
	}

	close(cIn)

	if err := <-out; err != ee {
		t.Error(err)
	}
}
