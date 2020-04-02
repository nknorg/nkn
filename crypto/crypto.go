package crypto

import (
	"fmt"

	"github.com/nknorg/nkn/crypto/ed25519"
)

var Sha256ZeroHash = make([]byte, 32)

func GenKeyPair() ([]byte, []byte, error) {
	return ed25519.GenKeyPair()
}

func GetPublicKeyFromPrivateKey(privateKey []byte) []byte {
	return ed25519.GetPublicKeyFromPrivateKey(privateKey)
}

func GetPrivateKeyFromSeed(seed []byte) []byte {
	return ed25519.GetPrivateKeyFromSeed(seed)
}

func GetSeedFromPrivateKey(priKey []byte) []byte {
	return ed25519.GetSeedFromPrivateKey(priKey)
}

func Sign(privateKey, data []byte) ([]byte, error) {
	return ed25519.Sign(privateKey, data)
}

func Verify(publicKey, data, signature []byte) error {
	return ed25519.Verify(publicKey, data, signature)
}

func GenerateVrf(privateKey, data []byte, randSrc bool) ([]byte, []byte, error) {
	return ed25519.GenerateVrf(privateKey, data, randSrc)
}

func VerifyVrf(publicKey, data, vrf, proof []byte) bool {
	return ed25519.VerifyVrf(publicKey, data, vrf, proof)
}

func CheckPublicKey(publicKey []byte) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size %d, expecting %d", len(publicKey), ed25519.PublicKeySize)
	}
	return nil
}

func CheckPrivateKey(privateKey []byte) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key size %d, expecting %d", len(privateKey), ed25519.PrivateKeySize)
	}
	return nil
}

func CheckSeed(seed []byte) error {
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("invalid seed size %d, expecting %d", len(seed), ed25519.SeedSize)
	}
	return nil
}
