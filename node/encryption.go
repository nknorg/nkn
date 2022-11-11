package node

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// for encryption
const (
	nonceSize     = 24
	SharedKeySize = 32
)

var EmptyNonce [nonceSize]byte

func EncryptMessage(message []byte, sharedKey *[SharedKeySize]byte) []byte {
	if sharedKey == nil {
		return append(EmptyNonce[:], message...)
	}

	var nonce [nonceSize]byte
	_, err := rand.Read(nonce[:])
	if err != nil {
		return nil
	}

	encrypted := make([]byte, len(message)+box.Overhead+nonceSize)
	copy(encrypted[:nonceSize], nonce[:])
	box.SealAfterPrecomputation(encrypted[nonceSize:nonceSize], message, &nonce, sharedKey)

	return encrypted
}

func DecryptMessage(message []byte, sharedKey *[SharedKeySize]byte) ([]byte, bool, error) {
	if len(message) < nonceSize {
		return nil, false, fmt.Errorf("encrypted message should have at least %d bytes", nonceSize)
	}

	var nonce [nonceSize]byte
	copy(nonce[:], message[:nonceSize])
	if nonce == EmptyNonce {
		return message[nonceSize:], false, nil
	}

	if sharedKey == nil {
		return nil, false, errors.New("cannot decrypt message: no shared key yet")
	}

	if len(message) < nonceSize+box.Overhead {
		return nil, false, fmt.Errorf("encrypted message should have at least %d bytes", nonceSize+box.Overhead)
	}

	decrypted := make([]byte, len(message)-nonceSize-box.Overhead)
	_, ok := box.OpenAfterPrecomputation(decrypted[:0], message[nonceSize:], &nonce, sharedKey)
	if !ok {
		return nil, false, errors.New("decrypt message failed")
	}

	return decrypted, true, nil
}
