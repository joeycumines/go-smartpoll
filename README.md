# go-smartpoll

Package smartpoll offers dynamic, reactive scheduling for synchronized polling of multiple data points.
It provides a highly configurable "control loop" (roughly `for { ... select ...}`), a scheduler API, and concurrency
control, for task execution.

See the [API docs](https://pkg.go.dev/github.com/joeycumines/go-smartpoll).

## Features

- **Synchronized Polling**: Smartpoll allows you to manage multiple data points in a synchronized manner. This is
  particularly useful when you need to aggregate information from multiple sources.

- **Dynamic Scheduling**: The package provides a dynamic scheduling mechanism. This means that you can adjust the
  schedule of polling operations on the fly, based on the needs of your application.

- **Reactive**: Smartpoll is reactive. It responds to changes in the state of your application and adjusts the polling
  operations accordingly.

- **Easy to Implement**: The package is designed to make it easy to implement a control loop. This reduces the
  complexity of your code and makes it easier to maintain.

## Examples

### 1. Aggregating Data from Multiple APIs

Imagine you are building a weather application, that aggregates data from multiple weather APIs, to provide a more
accurate forecast.
Your backend to retrieve the data, and perform the aggregation, is a simple, single-instance worker.
Each API has different rate limits, and you want to ensure you're not hitting these limits.
You also want to be able to implement backoff/retry, per API.
Each time any of the input data changes, the aggregate will be regenerated, and the result published.

```go
// TODO: Actually implement this, as a runnable example.
// Assume state in the local scope, without explicit synchronisation, except where noted.

weatherAPI1Task := func(ctx context.Context) (smartpoll.TaskHook, error) {
	// running in a separate goroutine...
	// assume backoff / retries baked into retrieving the result
	result, err := // ...
	if err != nil {
		// fatal error, will terminate the control loop
        return nil, err
    }

	return func(ctx context.Context, internal *Internal) error {
		// synchronised with the control loop...

		// reschedule as desired
		internal.ScheduleSooner("weatherAPI1", time.Second * 10)

		if !result.Equal(lastResult) {
			// schedule a publish task, if not already scheduled
			internal.Schedule("publish", 0)
        }

		lastResult = result
		return nil
    }, nil
}

publishTask := func(ctx context.Context) (smartpoll.TaskHook, error) {
    // we are running in a separate goroutine - this might use a mutex, atomic, or some other mechanism to synchronise
	allDataSnapshot := getAllDataSynchronised()

	// this could also be made available to the other tasks, e.g. updated to a variable in the parent scope, in a TaskHook
	aggregateResult := transformAllData(allDataSnapshot)

	// ... perform IO etc, to publish the aggregate result

    return nil, nil // TaskHook is omitted - nothing to synchronise with the control loop
}

scheduler, _ := smartpoll.New(
	smartpoll.WithRunHook(func(ctx context.Context, internal *smartpoll.Internal) error {
		// schedule the first invocation of each polling task, after which they manage their own lifecycle
		internal.Schedule("weatherAPI1", 0)
		internal.Schedule("weatherAPI2", 0)
		internal.Schedule("weatherAPI3", 0)
    }),
    smartpoll.WithTask("weatherAPI1", weatherAPI1Task),
    smartpoll.WithTask("weatherAPI2", weatherAPI2Task),
    smartpoll.WithTask("weatherAPI3", weatherAPI3Task),
	smartpoll.WithTask("publish", publishTask), // scheduled by the
)

scheduler.Run(context.Background())
```

In each task, you can adjust the schedule based on the rate limits of each API.

## Litmus test

Answering yes to one or more of the following might indicate that smartpoll is a good fit for your use case.

1. Do you need to control (schedule or run) distinct tasks, which operate in the background?
2. Do you need a mechanism to synchronise handling the results of tasks?
3. Do you need tasks to be able to coordinate with or schedule other tasks, in an arbitrary manner?
4. Do you need tasks which run on a dynamic interval?
5. Do you need to be able to reschedule tasks, or inspect when they are scheduled to run?
6. Do you also need to be able to perform arbitrary blocking logic (including accessing task results and scheduling
   tasks), in response to arbitrary events, scheduled in a "fair" manner, alongside the built-in behavior?
7. Do you need to be able to handle fatal errors, from tasks, or any logic running within the control loop?
8. Do you need to be able to tear down then later restart the control loop, e.g. in response to arbitrary events?

Smartpoll DOES NOT provide, but could be used with an implementation which provides the following. The effort involved
varies.

- Cancellation of running tasks (trivial)
- Waiting for all tasks to exit on return of `Scheduler.Run` (trivial, and there IS support for waiting on re-run)
- Built-in prioritisation or other higher-level strategies to order independent tasks (non-trivial)
- The ability to schedule a given task more than once, in addition to any running invocation of that task (somewhat
  non-trivial)
- Cron-based scheduling or similar (trivial, given a suitable cron implementation)

Smartpoll does not support:

- Dynamically adding or removing supported tasks (potentially able to be worked around by using a new `Scheduler`)
