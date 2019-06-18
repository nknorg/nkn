package vm

import (
	"crypto/sha256"
	"errors"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
)

type ECCrypto struct {
}

func (c *ECCrypto) Hash160(message []byte) []byte {
	temp, _ := common.ToCodeHash(message)
	return temp.ToArray()
}

func (c *ECCrypto) Hash256(message []byte) []byte {
	return []byte{}
}

func (c *ECCrypto) VerifySignature(message []byte, signature []byte, pubkey []byte) (bool, error) {
	pk, err := crypto.DecodePoint(pubkey)
	if err != nil {
		return false, errors.New("[ECCrypto], crypto.DecodePoint failed.")
	}

	digest := sha256.Sum256(message)
	err = crypto.Verify(*pk, digest[:], signature)
	if err != nil {
		return false, errors.New("[ECCrypto], VerifySignature failed.")
	}

	return true, nil
}
