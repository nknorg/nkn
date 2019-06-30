package crypto

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto/ed25519"
	"github.com/nknorg/nkn/crypto/util"
)

var Sha256ZeroHash = make([]byte, 32)

type PubKey struct {
	X, Y *big.Int
}

func GenKeyPair() ([]byte, PubKey, error) {
	mPubKey := new(PubKey)
	var privateD []byte
	var X *big.Int
	var Y *big.Int
	var err error

	privateD, X, Y, err = ed25519.GenKeyPair()

	if nil != err {
		return nil, *mPubKey, err
	}

	privkey := privateD
	mPubKey.X = new(big.Int).Set(X)
	mPubKey.Y = new(big.Int).Set(Y)
	return privkey, *mPubKey, nil
}

func GetPrivateKeyFromSeed(seed []byte) []byte {
	return ed25519.GetPrivateKeyFromSeed(seed)
}

func GetSeedFromPrivateKey(priKey []byte) []byte {
	return ed25519.GetSeedFromPrivateKey(priKey)
}

func Sign(privateKey []byte, data []byte) ([]byte, error) {
	var r *big.Int
	var s *big.Int
	var err error

	r, s, err = ed25519.Sign(privateKey, data)

	if err != nil {
		return nil, err
	}

	signature := make([]byte, util.SIGNATURELEN)

	lenR := len(r.Bytes())
	lenS := len(s.Bytes())
	copy(signature[util.SIGNRLEN-lenR:], r.Bytes())
	copy(signature[util.SIGNATURELEN-lenS:], s.Bytes())
	return signature, nil
}

func Verify(publicKey PubKey, data []byte, signature []byte) error {
	len := len(signature)
	if len != util.SIGNATURELEN {
		return errors.New("Unknown signature length")
	}

	r := new(big.Int).SetBytes(signature[:len/2])
	s := new(big.Int).SetBytes(signature[len/2:])

	return ed25519.Verify(publicKey.X, publicKey.Y, data, r, s)
}

func GenerateVrf(privateKey, data []byte, randSrc bool) ([]byte, []byte, error) {
	return ed25519.GenerateVrf(privateKey, data, randSrc)
}

func VerifyVrf(publicKey PubKey, data, vrf, proof []byte) bool {
	return ed25519.VerifyVrf(publicKey.X, publicKey.Y, data, vrf, proof)
}

func (e *PubKey) Serialize(w io.Writer) error {
	if err := serialization.WriteVarBytes(w, e.X.Bytes()); err != nil {
		return err
	}
	if err := serialization.WriteVarBytes(w, e.Y.Bytes()); err != nil {
		return err
	}
	return nil
}

func (e *PubKey) Deserialize(r io.Reader) error {
	bufX, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	e.X = big.NewInt(0)
	e.X = e.X.SetBytes(bufX)

	bufY, err := serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}
	e.Y = big.NewInt(0)
	e.Y = e.Y.SetBytes(bufY)

	return nil
}

func CheckPrivateKey(privkey []byte) error {
	if len(privkey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key size %d, expecting %d", len(privkey), ed25519.PrivateKeySize)
	}

	return nil
}

func CheckSeed(seed []byte) error {
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("invalid seed size %d, expecting %d", len(seed), ed25519.SeedSize)
	}

	return nil
}

type PubKeySlice []*PubKey

func (p PubKeySlice) Len() int { return len(p) }
func (p PubKeySlice) Less(i, j int) bool {
	r := p[i].X.Cmp(p[j].X)
	if r <= 0 {
		return true
	}
	return false
}
func (p PubKeySlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func Sha256(value []byte) []byte {
	data := make([]byte, 32)
	digest := sha256.Sum256(value)
	copy(data, digest[0:32])
	return data
}

func Equal(e1 *PubKey, e2 *PubKey) bool {
	r := e1.X.Cmp(e2.X)
	if r != 0 {
		return false
	}
	r = e1.Y.Cmp(e2.Y)
	if r == 0 {
		return true
	}
	return false
}
