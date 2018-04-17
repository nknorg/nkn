package util

import (
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	//"math/big"
)

const (
	HASHLEN       = 32
	PRIVATEKEYLEN = 32
	PUBLICKEYLEN  = 32
	SIGNRLEN      = 32
	SIGNSLEN      = 32
	SIGNATURELEN  = 64
	NEGBIGNUMLEN  = 33
)

const (
	MaxRandomBytesLength = 2048
)

type CryptoAlgSet struct {
	EccParams elliptic.CurveParams
	Curve     elliptic.Curve
}

func RandomBytes(length int) []byte {
	if length <= 0 || length >= MaxRandomBytesLength {
		return nil
	}
	random := make([]byte, length)
	rand.Read(random)

	return random
}

func Hash(data []byte) [HASHLEN]byte {
	return sha256.Sum256(data)
}

// CheckMAC reports whether messageMAC is a valid HMAC tag for message.
func CheckMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

func RIPEMD160(value []byte) []byte {
	//TODO: implement RIPEMD160

	return nil
}
