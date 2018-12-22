package timer

import "time"

// StopTimer stops a timer and clear out the channel if not yet
func StopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// ResetTimer stops and resets a timer
func ResetTimer(timer *time.Timer, duration time.Duration) {
	StopTimer(timer)
	timer.Reset(duration)
}
