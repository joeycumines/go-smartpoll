//go:build !go1.23

package smartpoll

import "time"

// Go 1.23 introduced breaking changes to the time.Timer.Stop API.
func stopAndDrainTimer(t *time.Timer) (already bool) {
	already = t.Stop()
	if !already {
		select {
		case <-t.C:
		default:
			already = true
		}
	}
	return
}
