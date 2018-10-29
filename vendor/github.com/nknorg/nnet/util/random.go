package util

import "crypto/rand"

// RandBytes returns a random []byte with length l
func RandBytes(len int) ([]byte, error) {
	b := make([]byte, len)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
