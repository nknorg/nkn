package crypto

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/crypto/ed25519"
	"github.com/nknorg/nkn/crypto/p256r1"
	"github.com/nknorg/nkn/crypto/util"
)

const (
	P256R1  = 0
	Ed25519 = 1
)

//It can be P256R1
var AlgChoice int

var algSet util.CryptoAlgSet

type PubKey struct {
	X, Y *big.Int
}

func SetAlg(algChoice string) {
	switch algChoice {
	case "Ed25519":
		AlgChoice = Ed25519
		ed25519.Init(&algSet)
	case "P256R1":
		AlgChoice = P256R1
		p256r1.Init(&algSet)
	default:
		panic("unsupported algorithm type")
	}

	return
}

func GenKeyPair() ([]byte, PubKey, error) {
	mPubKey := new(PubKey)
	var privateD []byte
	var X *big.Int
	var Y *big.Int
	var err error

	if Ed25519 == AlgChoice {
		privateD, X, Y, err = ed25519.GenKeyPair(&algSet)
	} else {
		privateD, X, Y, err = p256r1.GenKeyPair(&algSet)
	}

	if nil != err {
		return nil, *mPubKey, err
	}

	if Ed25519 == AlgChoice {
		privkey := privateD
		mPubKey.X = new(big.Int).Set(X)
		mPubKey.Y = new(big.Int).Set(Y)
		return privkey, *mPubKey, nil
	} else {
		privkey := make([]byte, util.PRIVATEKEYLEN)
		copy(privkey[util.PRIVATEKEYLEN-len(privateD):], privateD)

		mPubKey.X = new(big.Int).Set(X)
		mPubKey.Y = new(big.Int).Set(Y)
		return privkey, *mPubKey, nil
	}

}

func Sign(privateKey []byte, data []byte) ([]byte, error) {
	var r *big.Int
	var s *big.Int
	var err error

	if Ed25519 == AlgChoice {
		r, s, err = ed25519.Sign(&algSet, privateKey, data)
	} else {
		r, s, err = p256r1.Sign(&algSet, privateKey, data)
	}

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
		fmt.Printf("Unknown signature length %d\n", len)
		return errors.New("Unknown signature length")
	}

	r := new(big.Int).SetBytes(signature[:len/2])
	s := new(big.Int).SetBytes(signature[len/2:])

	if Ed25519 == AlgChoice {
		return ed25519.Verify(&algSet, publicKey.X, publicKey.Y, data, r, s)
	}

	return p256r1.Verify(&algSet, publicKey.X, publicKey.Y, data, r, s)
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
