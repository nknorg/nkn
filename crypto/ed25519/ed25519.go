package ed25519

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/nknorg/nkn/crypto/util"

	"golang.org/x/crypto/ed25519"
)

const (
	ED25519_PUBLICKEYSIZE  = 32
	ED25519_PRIVATEKEYSIZE = 64
	ED25519_SIGNATURESIZE  = 64
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
	//privKey := [64]byte{}
	//copy(privKey[:], sk.SK[0:64])
	sig := ed25519.Sign(priKey, data)
	return new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]), nil
}

func Verify(algSet *util.CryptoAlgSet, X *big.Int, Y *big.Int, data []byte, r, s *big.Int) error {
	//pubKey := [32]byte{}
	//sig := [64]byte{}
	//copy(pubKey[:], pk.PK[0:32])
	//copy(sig[:], signature[0:64])
	pubKey := X.Bytes()
	sig := r.Bytes()
	sig = append(sig, s.Bytes()...)
	if !ed25519.Verify(pubKey, data, sig) {
		return errors.New("ED25519PubKey.Verify: failed.")
	}

	return nil
}

func NewKeyFromPrivkey(privKey []byte) *big.Int {
	seed := privKey[:32]
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := make([]byte, ed25519.PublicKeySize)
	copy(publicKey, privateKey[32:])
	X := new(big.Int).SetBytes(publicKey)

	return X
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
