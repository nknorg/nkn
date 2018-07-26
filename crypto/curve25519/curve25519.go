package curve25519

import (
	"crypto/ecdsa"
	. "crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/nknorg/nkn/crypto/util"
	"encoding/hex"
)

var initonce sync.Once

type curve25519Curve struct {
	*CurveParams
}

var (
	curve25519            curve25519Curve
)

func initCurve25519() {
	curve25519.CurveParams = &CurveParams{Name: "Curve25519"}
	curve25519.P, _ = new(big.Int).SetString("57896044618658097711785492504343953926634992332820282019728792003956564819949", 10)
	curve25519.N, _ = new(big.Int).SetString("7237005577332262213973186563042994240857116359379907606001950938285454250989", 10)
	curve25519.B, _ = new(big.Int).SetString("00", 16)
	curve25519.Gx, _ = new(big.Int).SetString("0900000000000000000000000000000000000000000000000000000000000000", 16)
	curve25519.Gy, _ = new(big.Int).SetString("20ae19a1b8a086b4e01edd2c7748d14c923d4d7e6d7c61b229e9c5a27eced3d9", 16)
	curve25519.BitSize = 256
}

func Curve25519() Curve {
	initonce.Do(initCurve25519)
	return curve25519
}

func (curve curve25519Curve) Params() *CurveParams {
	return curve.CurveParams
}

func Init(algSet *util.CryptoAlgSet) {
	algSet.Curve = Curve25519()
	algSet.EccParams = *(algSet.Curve.Params())
}

func GenKeyPair(algSet *util.CryptoAlgSet) ([]byte, *big.Int, *big.Int, error) {
	privateKey := new(ecdsa.PrivateKey)
	privateKey, err := ecdsa.GenerateKey(algSet.Curve, rand.Reader)
	if err != nil {
		return nil, nil, nil, errors.New("Generate key pair error")
	}

	priKey := privateKey.D.Bytes()
	print(hex.EncodeToString(priKey))
	return priKey, privateKey.PublicKey.X, privateKey.PublicKey.Y, nil
}

func Sign(algSet *util.CryptoAlgSet, priKey []byte, data []byte) (*big.Int, *big.Int, error) {
	digest := util.Hash(data)

	privateKey := new(ecdsa.PrivateKey)
	privateKey.Curve = algSet.Curve
	privateKey.D = big.NewInt(0)
	privateKey.D.SetBytes(priKey)

	r := big.NewInt(0)
	s := big.NewInt(0)

	r, s, err := ecdsa.Sign(rand.Reader, privateKey, digest[:])
	if err != nil {
		fmt.Printf("Sign error\n")
		return nil, nil, err
	}
	return r, s, nil
}

func Verify(algSet *util.CryptoAlgSet, X *big.Int, Y *big.Int, data []byte, r, s *big.Int) error {
	digest := util.Hash(data)

	pub := new(ecdsa.PublicKey)
	pub.Curve = algSet.Curve

	pub.X = new(big.Int).Set(X)
	pub.Y = new(big.Int).Set(Y)

	if ecdsa.Verify(pub, digest[:], r, s) {
		return nil
	} else {
		return errors.New("[Validation], Verify failed.")
	}
}

