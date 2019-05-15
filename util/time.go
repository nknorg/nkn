package util

import (
	"math/rand"
	"time"
)

// RandDuration generates a random duration
func RandDuration(average time.Duration, delta float64) time.Duration {
	// uniform random number between (1 - delta) * average to (1 + delta) * average
	return time.Duration((1 - delta + 2*rand.Float64()*delta) * float64(average))
}
