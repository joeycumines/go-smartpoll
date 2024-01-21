package smartpoll

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	defer checkNumGoroutines(time.Second * 3)(t)

	t.Run("no tasks configured", func(t *testing.T) {
		_, err := New()
		if err == nil || err.Error() != "smartpoll: no tasks configured" {
			t.Errorf("expected error not found: %v", err)
		}
	})

	t.Run("task func is nil", func(t *testing.T) {
		_, err := New(WithTask("task1", nil))
		if err == nil || err.Error() != "smartpoll: task func must not be nil" {
			t.Errorf("expected error not found: %v", err)
		}
	})

	t.Run("task key is not comparable", func(t *testing.T) {
		_, err := New(WithTask([]int{1, 2, 3}, func(ctx context.Context) (TaskHook, error) {
			return nil, nil
		}))
		if err == nil || err.Error() != "smartpoll: task key type []int is not comparable" {
			t.Errorf("expected error not found: %v", err)
		}
	})

	t.Run("hook channel is nil", func(t *testing.T) {
		_, err := New(WithHook[int](nil, func(ctx context.Context, internal *Internal, value int, ok bool) error {
			return nil
		}))
		if err == nil || err.Error() != "smartpoll: hook channel must not be nil" {
			t.Errorf("expected error not found: %v", err)
		}
	})

	t.Run("hook func is nil", func(t *testing.T) {
		ch := make(chan int)
		defer close(ch)
		_, err := New(WithHook[int](ch, nil))
		if err == nil || err.Error() != "smartpoll: hook func must not be nil" {
			t.Errorf("expected error not found: %v", err)
		}
	})

	t.Run("run hook is nil", func(t *testing.T) {
		_, err := New(WithRunHook(nil))
		if err == nil || err.Error() != "smartpoll: run hook must not be nil" {
			t.Errorf("expected error not found: %v", err)
		}
	})

	t.Run("too many cases", func(t *testing.T) {
		options := make([]Option, maxSelectCases-numInternalCases+1)
		for i := range options {
			options[i] = WithTask(i, func(ctx context.Context) (TaskHook, error) {
				return nil, nil
			})
		}
		_, err := New(options...)
		if err == nil || err.Error() != "smartpoll: init internal cases too many cases: 65537 > 65536" {
			t.Errorf("expected error not found: %v", err)
		}
	})
}
