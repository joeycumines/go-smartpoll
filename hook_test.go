package smartpoll

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestHook_call_withValidValue(t *testing.T) {
	hook := Hook[int](func(ctx context.Context, internal *Internal, value int, ok bool) error {
		if value != 10 {
			t.Fatal(`unexpected value`)
		}
		if !ok {
			t.Fatal(`unexpected ok`)
		}
		return nil
	})

	ctx := context.Background()
	internal := new(Internal)
	value := reflect.ValueOf(10)

	if err := hook.call(ctx, internal, value, true); err != nil {
		t.Fatal(err)
	}
}

func TestHook_call_withNilValue(t *testing.T) {
	hook := Hook[int](func(ctx context.Context, internal *Internal, value int, ok bool) error {
		if value != 0 {
			t.Fatal(`unexpected value`)
		}
		if ok {
			t.Fatal(`unexpected ok`)
		}
		return nil
	})

	ctx := context.Background()
	internal := new(Internal)
	value := reflect.ValueOf(nil)

	if err := hook.call(ctx, internal, value, false); err != nil {
		t.Fatal(err)
	}
}

func TestHook_call_withError(t *testing.T) {
	expectedError := errors.New(`test error`)
	hook := Hook[int](func(ctx context.Context, internal *Internal, value int, ok bool) error {
		return expectedError
	})

	ctx := context.Background()
	internal := new(Internal)
	value := reflect.ValueOf(10)

	if err := hook.call(ctx, internal, value, true); err != expectedError {
		t.Fatal(`unexpected error`)
	}
}

func TestHook_call_any(t *testing.T) {
	type Args struct {
		ctx      context.Context
		internal *Internal
		value    any
		ok       bool
	}
	var args *Args
	var result error
	var hook Hook[any] = func(ctx context.Context, internal *Internal, value any, ok bool) error {
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
		if args != nil {
			t.Fatal(`unexpected call`)
		}
		args = &Args{ctx, internal, value, ok}
		return result
	}

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, 123)
	internal := new(Internal)
	var value any

	ch := make(chan any, 1)

	var rv reflect.Value
	var ok bool
	setValue := func(v any) {
		value = v
		ch <- value
		_, rv, ok = reflect.Select([]reflect.SelectCase{{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}})
	}

	check := func() {
		t.Helper()
		args = nil
		if err := hook.call(ctx, internal, rv, ok); err != result {
			t.Fatal(err)
		}
		if args == nil {
			t.Fatal(`expected call`)
		}
		if !func() (equal bool) {
			var success bool
			defer func() {
				recover()
				if !success {
					equal = reflect.DeepEqual(args.value, value)
				}
			}()
			equal = args.value == value
			success = true
			return
		}() {
			t.Fatal(`unexpected value`, value)
		}
		if args.ok != ok {
			t.Fatal(`unexpected ok`, ok)
		}
		if args.ctx == ctx || args.ctx.Value(ctxKey{}) != 123 || args.ctx.Err() == nil {
			t.Fatal(`unexpected ctx`, ctx)
		}
		if args.internal != internal {
			t.Fatal(`unexpected internal`, internal)
		}
	}

	setValue(nil)
	check()

	result = errors.New(`test error`)
	check()

	setValue(5)
	check()

	result = nil
	check()

	setValue((map[int]bool)(nil))
	check()

	setValue(map[int]bool{})
	check()

	setValue(map[int]bool{645: true})
	check()

	setValue(&struct{}{})
	check()
}

func TestHook_call_error(t *testing.T) {
	type Args struct {
		ctx      context.Context
		internal *Internal
		value    error
		ok       bool
	}
	var args *Args
	var result error
	var hook Hook[error] = func(ctx context.Context, internal *Internal, value error, ok bool) error {
		if args != nil {
			t.Fatal(`unexpected call`)
		}
		args = &Args{ctx, internal, value, ok}
		return result
	}

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, 123)
	internal := new(Internal)
	var value error

	ch := make(chan error, 1)

	var rv reflect.Value
	var ok bool
	setValue := func(v error) {
		value = v
		ch <- value
		_, rv, ok = reflect.Select([]reflect.SelectCase{{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}})
	}

	check := func() {
		t.Helper()
		args = nil
		if err := hook.call(ctx, internal, rv, ok); err != result {
			t.Fatal(err)
		}
		if args == nil {
			t.Fatal(`expected call`)
		}
		if args.value != value {
			t.Fatal(`unexpected value`, value)
		}
		if args.ok != ok {
			t.Fatal(`unexpected ok`, ok)
		}
		if args.ctx == ctx || args.ctx.Value(ctxKey{}) != 123 {
			t.Fatal(`unexpected ctx`, ctx)
		}
		if args.internal != internal {
			t.Fatal(`unexpected internal`, internal)
		}
	}

	setValue(nil)
	check()

	result = errors.New(`test error`)
	check()

	setValue(errors.New(`some error`))
	check()

	result = nil
	check()
}

func TestHook_call_pointer(t *testing.T) {
	type Args struct {
		ctx      context.Context
		internal *Internal
		value    *struct{}
		ok       bool
	}
	var args *Args
	var result error
	var hook Hook[*struct{}] = func(ctx context.Context, internal *Internal, value *struct{}, ok bool) error {
		if args != nil {
			t.Fatal(`unexpected call`)
		}
		args = &Args{ctx, internal, value, ok}
		return result
	}

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, 123)
	internal := new(Internal)
	var value *struct{}

	ch := make(chan *struct{}, 1)

	var rv reflect.Value
	var ok bool
	setValue := func(v *struct{}) {
		value = v
		ch <- value
		_, rv, ok = reflect.Select([]reflect.SelectCase{{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}})
	}

	check := func() {
		t.Helper()
		args = nil
		if err := hook.call(ctx, internal, rv, ok); err != result {
			t.Fatal(err)
		}
		if args == nil {
			t.Fatal(`expected call`)
		}
		if args.value != value {
			t.Fatal(`unexpected value`, value)
		}
		if args.ok != ok {
			t.Fatal(`unexpected ok`, ok)
		}
		if args.ctx == ctx || args.ctx.Value(ctxKey{}) != 123 {
			t.Fatal(`unexpected ctx`, ctx)
		}
		if args.internal != internal {
			t.Fatal(`unexpected internal`, internal)
		}
	}

	setValue(nil)
	check()

	result = errors.New(`test error`)
	check()

	setValue(new(struct{}))
	check()

	result = nil
	check()

	setValue(new(struct{}))
	check()
}
