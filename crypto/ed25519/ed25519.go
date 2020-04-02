package ed25519

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/nknorg/nkn/crypto/ed25519/extra25519"
	"github.com/nknorg/nkn/crypto/ed25519/vrf"

	"golang.org/x/crypto/ed25519"
)

const PublicKeySize = ed25519.PublicKeySize
const PrivateKeySize = ed25519.PrivateKeySize
const SignatureSize = ed25519.SignatureSize
const SeedSize = ed25519.SeedSize

func GenKeyPair() ([]byte, []byte, error) {
	return ed25519.GenerateKey(rand.Reader)
}

func Sign(privateKey, data []byte) ([]byte, error) {
	if len(privateKey) != PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size %d", len(privateKey))
	}
	return ed25519.Sign(privateKey, data), nil
}

func Verify(publicKey, data, signature []byte) error {
	if len(publicKey) != PublicKeySize {
		return fmt.Errorf("invalid public key size %d", len(publicKey))
	}
	if !ed25519.Verify(publicKey, data, signature) {
		return errors.New("invalid signature")
	}
	return nil
}

func GetPublicKeyFromPrivateKey(privateKey []byte) []byte {
	seed := privateKey[:ed25519.SeedSize]
	privateKey = ed25519.NewKeyFromSeed(seed)
	publicKey := make([]byte, ed25519.PublicKeySize)
	copy(publicKey, privateKey[ed25519.SeedSize:])
	return publicKey
}

func GetSeedFromPrivateKey(priKey []byte) []byte {
	privateKey := ed25519.PrivateKey(priKey)
	return privateKey.Seed()
}

func GetPrivateKeyFromSeed(seed []byte) []byte {
	return ed25519.NewKeyFromSeed(seed)
}

func GenerateVrf(privateKey, data []byte, randSrc bool) ([]byte, []byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("invalid private key length")
	}
	sk := vrf.PrivateKey(privateKey)
	vrf, proof := sk.Prove(data, randSrc)
	return vrf, proof, nil
}

func VerifyVrf(publicKey, data, dataVrf, proof []byte) bool {
	pk := vrf.PublicKey(publicKey)
	return pk.Verify(data, dataVrf, proof)
}

func PrivateKeyToCurve25519PrivateKey(privateKey *[64]byte) *[32]byte {
	var curve25519PrivateKey [32]byte
	extra25519.PrivateKeyToCurve25519(&curve25519PrivateKey, privateKey)
	return &curve25519PrivateKey
}

func PublicKeyToCurve25519PublicKey(publicKey *[32]byte) (*[32]byte, bool) {
	var curve25519PublicKey [32]byte
	ok := extra25519.PublicKeyToCurve25519(&curve25519PublicKey, publicKey)
	return &curve25519PublicKey, ok
}
