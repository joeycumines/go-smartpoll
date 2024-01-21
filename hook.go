package smartpoll

import (
	"context"
	"reflect"
)

type (
	// Hook will be called with the values `value, ok := <-ch`, where ch
	// represents the channel this hook was configured with. The context will
	// be a descendent of the Scheduler.Run context, and will be cancelled
	// after the hook returns.
	Hook[T any] func(ctx context.Context, internal *Internal, value T, ok bool) error

	// RunHook is a hook which is called on each Scheduler.Run, just prior to
	// starting the main loop. The context will be a descendent of the
	// Scheduler.Run context, and will be cancelled after the hook returns.
	RunHook func(ctx context.Context, internal *Internal) error
)

func (x Hook[T]) call(ctx context.Context, internal *Internal, value reflect.Value, ok bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var v T
	if value.IsValid() && (value.Kind() != reflect.Interface || !value.IsNil()) {
		v = value.Interface().(T)
	}
	return x(ctx, internal, v, ok)
}

func (x RunHook) call(ctx context.Context, internal *Internal) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return x(ctx, internal)
}
