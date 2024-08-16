package smartpoll

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type (
	Option interface {
		applyOption(c *schedulerConfig) error
	}

	optionFunc func(c *schedulerConfig) error

	schedulerConfig struct {
		tasks    map[any]*taskState                                                                  // see Scheduler.tasks
		hooks    []func(ctx context.Context, internal *Internal, value reflect.Value, ok bool) error // see Scheduler.hooks
		cases    []reflect.SelectCase                                                                // see Scheduler.cases
		runHooks []RunHook                                                                           // see Scheduler.runHooks
	}
)

var (
	_ Option = optionFunc(nil)
)

// New initialises a [Scheduler], with the given options.
// See also `With*` prefixed functions.
func New(options ...Option) (*Scheduler, error) {
	c := schedulerConfig{
		tasks: make(map[any]*taskState),
	}

	for _, option := range options {
		if err := option.applyOption(&c); err != nil {
			return nil, err
		}
	}

	if len(c.tasks) == 0 {
		return nil, errors.New(`smartpoll: no tasks configured`)
	}

	x := Scheduler{
		tasks:          c.tasks,
		hooks:          c.hooks,
		cases:          c.cases,
		taskLockCh:     make(chan error),
		taskUnlockCh:   make(chan error),
		taskCompleteCh: make(chan struct{}, 1),
		runHooks:       c.runHooks,
	}

	if err := x.initCases(); err != nil {
		return nil, err
	}

	return &x, nil
}

// WithTask adds a task, identified by the given key. The provided task may
// be scheduled.
func WithTask(key any, task Task) Option {
	return optionFunc(func(c *schedulerConfig) (err error) {
		if task == nil {
			return errors.New(`smartpoll: task func must not be nil`)
		}
		var success bool
		defer func() {
			if !success {
				recover()
				err = fmt.Errorf(`smartpoll: task key type %T is not comparable`, key)
			}
		}()
		c.tasks[key] = &taskState{
			task: task,
		}
		success = true
		return nil
	})
}

// WithHook adds a [Hook], wired up to the given channel.
func WithHook[T any](ch <-chan T, hook Hook[T]) Option {
	return optionFunc(func(c *schedulerConfig) error {
		if ch == nil {
			return errors.New(`smartpoll: hook channel must not be nil`)
		}
		if hook == nil {
			return errors.New(`smartpoll: hook func must not be nil`)
		}
		c.cases = append(c.cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		})
		c.hooks = append(c.hooks, hook.call)
		return nil
	})
}

// WithRunHook adds a [RunHook] to be called on each [Scheduler.Run], just
// prior to starting the main loop. If more than one [RunHook] is configured,
// they will be called in the order they were configured.
func WithRunHook(hook RunHook) Option {
	return optionFunc(func(c *schedulerConfig) error {
		if hook == nil {
			return errors.New(`smartpoll: run hook must not be nil`)
		}
		c.runHooks = append(c.runHooks, hook)
		return nil
	})
}

func (x optionFunc) applyOption(c *schedulerConfig) error {
	return x(c)
}
