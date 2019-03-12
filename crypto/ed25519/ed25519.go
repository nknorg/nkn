package ed25519

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/nknorg/nkn/crypto/util"

	"golang.org/x/crypto/ed25519"
)

func Init(algSet *util.CryptoAlgSet) {
}

func GenKeyPair(algSet *util.CryptoAlgSet) ([]byte, *big.Int, *big.Int, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, errors.New("NewEd25519: Generate key pair error")
	}

	X := new(big.Int).SetBytes(pubKey)
	Y := big.NewInt(0)

	return privKey, X, Y, nil
}

func Sign(algSet *util.CryptoAlgSet, priKey []byte, data []byte) (*big.Int, *big.Int, error) {
	sig := ed25519.Sign(priKey, data)
	return new(big.Int).SetBytes(sig[:ed25519.SignatureSize/2]), new(big.Int).SetBytes(sig[ed25519.SignatureSize/2:]), nil
}

func Verify(algSet *util.CryptoAlgSet, X *big.Int, Y *big.Int, data []byte, r, s *big.Int) error {
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

func GetPublicKeySize() int {
	return ed25519.PublicKeySize
}

func GetPrivateKeySize() int {
	return ed25519.PrivateKeySize
}

func GetSignatureSize() int {
	return ed25519.SignatureSize
}
