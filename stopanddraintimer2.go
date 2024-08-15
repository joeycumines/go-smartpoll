//go:build go1.23

package smartpoll

import "time"

// Go 1.23 introduced breaking changes to the time.Timer.Stop API.
func stopAndDrainTimer(t *time.Timer) (already bool) {
	select {
	case <-t.C:
		return false
	default:
		return t.Stop()
	}
}
