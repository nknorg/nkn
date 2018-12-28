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

// Generates a random duration
func randDuration(average time.Duration) time.Duration {
	// uniform random number between 2/3 * average to 4/3 * average
	return time.Duration(2.0 / 3.0 * (1 + rand.Float64()) * float64(average))
}
