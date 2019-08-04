package ed25519

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/nknorg/nkn/crypto/ed25519/extra25519"
	"github.com/nknorg/nkn/crypto/ed25519/vrf"

	"golang.org/x/crypto/ed25519"
)

const PublicKeySize = ed25519.PublicKeySize
const PrivateKeySize = ed25519.PrivateKeySize
const SignatureSize = ed25519.SignatureSize
const SeedSize = ed25519.SeedSize

func GenKeyPair() ([]byte, *big.Int, *big.Int, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, errors.New("NewEd25519: Generate key pair error")
	}

	X := new(big.Int).SetBytes(pubKey)
	Y := big.NewInt(0)

	return privKey, X, Y, nil
}

func Sign(priKey []byte, data []byte) (*big.Int, *big.Int, error) {
	sig := ed25519.Sign(priKey, data)
	return new(big.Int).SetBytes(sig[:ed25519.SignatureSize/2]), new(big.Int).SetBytes(sig[ed25519.SignatureSize/2:]), nil
}

func Verify(X *big.Int, Y *big.Int, data []byte, r, s *big.Int) error {
	pk := X.Bytes()
	pubKey := [ed25519.PublicKeySize]byte{}
	copy(pubKey[ed25519.PublicKeySize-len(pk):], pk)

	sig := [ed25519.SignatureSize]byte{}
	sigR := r.Bytes()
	copy(sig[ed25519.SignatureSize/2-len(sigR):], sigR[:])
	sigS := s.Bytes()
	copy(sig[ed25519.SignatureSize-len(sigS):], sigS[:])

	if !ed25519.Verify(pubKey[:], data, sig[:]) {
		return errors.New("Ed25519 PubKey Verify: failed.")
	}

	return nil
}

func NewKeyFromPrivkey(privKey []byte) *big.Int {
	seed := privKey[:ed25519.SeedSize]
	privateKey := ed25519.NewKeyFromSeed(seed)

	publicKey := make([]byte, ed25519.PublicKeySize)
	copy(publicKey, privateKey[ed25519.SeedSize:])
	pubKey := new(big.Int).SetBytes(publicKey)

	return pubKey
}

func GetSeedFromPrivateKey(priKey []byte) []byte {
	privateKey := ed25519.PrivateKey(priKey)
	return privateKey.Seed()
}

func GetPrivateKeyFromSeed(seed []byte) []byte {
	return ed25519.NewKeyFromSeed(seed)
}

func GenerateVrf(privKey, data []byte, randSrc bool) (dataVrf []byte, proof []byte, err error) {
	if len(privKey) != ed25519.PrivateKeySize {
		err = errors.New("bad length of privKey")
		return
	}
	sk := vrf.PrivateKey(privKey)
	dataVrf, proof = sk.Prove(data, randSrc)
	return
}

func VerifyVrf(X *big.Int, Y *big.Int, data, dataVrf, proof []byte) bool {
	publicKey := X.Bytes()
	pubKey := [ed25519.PublicKeySize]byte{}
	copy(pubKey[ed25519.PublicKeySize-len(publicKey):], publicKey)

	pk := vrf.PublicKey(pubKey[:])

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
