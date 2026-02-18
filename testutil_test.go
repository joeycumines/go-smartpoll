package smartpoll

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// checkNumGoroutines is intended to be used to check for errant goroutines,
// like `defer checkNumGoroutines(time.Second * 3)(t)`.
func checkNumGoroutines(max time.Duration) func(t *testing.T) {
	before := runtime.NumGoroutine()
	return func(t *testing.T) {
		if t != nil {
			t.Helper()
		}
		after := waitNumGoroutines(max, func(n int) bool { return n <= before })
		if after > before {
			var b bytes.Buffer
			_ = pprof.Lookup("goroutine").WriteTo(&b, 1)
			testingErrorfOrPanic(t, "%s\n\nstarted with %d goroutines finished with %d", b.Bytes(), before, after)
		}
	}
}

// waitNumGoroutines will block until there are a target number of goroutines
// remaining, or a max duration is exceeded.
func waitNumGoroutines(maxDur time.Duration, fn func(n int) bool) (n int) {
	const minDur = time.Millisecond * 10
	if maxDur < minDur {
		maxDur = minDur
	}
	count := int(maxDur / minDur)
	maxDur /= time.Duration(count)
	n = runtime.NumGoroutine()
	for i := 0; i < count && !fn(n); i++ {
		time.Sleep(maxDur)
		runtime.GC()
		n = runtime.NumGoroutine()
	}
	return
}

func testingErrorfOrPanic(t *testing.T, format string, values ...any) {
	if t == nil {
		panic(fmt.Errorf(format, values...))
	}
	t.Helper()
	t.Errorf(format, values...)
}
