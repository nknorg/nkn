package moca

import (
	"encoding/binary"
	"math/rand"
	"time"
)

// heiheightToKey convert block height to byte array
func heightToKey(height uint32) []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, height)
	return key
}

// randDuration generates a random duration
func randDuration(average time.Duration, delta float64) time.Duration {
	// uniform random number between (1 - delta) * average to (1 + delta) * average
	return time.Duration((1 - delta + 2*rand.Float64()*delta) * float64(average))
}
