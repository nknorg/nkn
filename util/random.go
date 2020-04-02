package util

import (
	"crypto/rand"
)

func RandomBytes(length int) []byte {
	if length <= 0 {
		return nil
	}
	random := make([]byte, length)
	rand.Read(random)
	return random
}
