package util

import (
	"crypto/rand"
)

const (
	SIGNRLEN     = 32
	SIGNSLEN     = 32
	SIGNATURELEN = 64
)

const (
	MaxRandomBytesLength = 2048
)

func RandomBytes(length int) []byte {
	if length <= 0 || length >= MaxRandomBytesLength {
		return nil
	}
	random := make([]byte, length)
	rand.Read(random)

	return random
}
