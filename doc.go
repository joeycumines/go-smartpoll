// Package smartpoll offers dynamic, reactive scheduling for synchronized
// polling of multiple data points. See the [README] for the rationale, use
// cases, and examples.
//
// The purpose of this implementation is to make it as easy as possible to
// implement a "control loop", which directly manages the scheduling of polling
// operations, and propagates and/or transforms the results. It achieves this
// by running all such operations, termed "hooks", within a single goroutine.
// The implementation consists of a Scheduler, which is configured with a set
// of Task values, with a corresponding identifying key, and optionally a set
// of Hook values, with which each receive from a corresponding channel. Each
// Task consists of a two-stage, higher-order function. The first stage is
// performed in its own goroutine, while the second stage is synchronized with
// the main loop. Channels for each custom Hook are selected alongside the
// logic for scheduling and handling the results of each Task.
//
// [README]: https://github.com/joeycumines/go-smartpoll/blob/main/README.md
package smartpoll
