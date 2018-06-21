package vault

import (
	"errors"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/crypto"
	. "github.com/nknorg/nkn/errors"
)

type Account struct {
	PrivateKey  []byte
	PublicKey   *crypto.PubKey
	ProgramHash Uint160
}

func NewAccount() (*Account, error) {
	priKey, pubKey, _ := crypto.GenKeyPair()
	signatureRedeemScript, err := contract.CreateSignatureRedeemScript(&pubKey)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "CreateSignatureRedeemScript failed")
	}
	programHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "ToCodeHash failed")
	}
	return &Account{
		PrivateKey:  priKey,
		PublicKey:   &pubKey,
		ProgramHash: programHash,
	}, nil
}

func NewAccountWithPrivatekey(privateKey []byte) (*Account, error) {
	privKeyLen := len(privateKey)
	if privKeyLen != 32 {
		return nil, errors.New("invalid private key length")
	}
	pubKey := crypto.NewPubKey(privateKey)
	signatureRedeemScript, err := contract.CreateSignatureRedeemScript(pubKey)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "CreateSignatureRedeemScript failed")
	}
	programHash, err := ToCodeHash(signatureRedeemScript)
	if err != nil {
		return nil, NewDetailErr(err, ErrNoCode, "ToCodeHash failed")
	}
	return &Account{
		PrivateKey:  privateKey,
		PublicKey:   pubKey,
		ProgramHash: programHash,
	}, nil
}

func (a *Account) PrivKey() []byte {
	return a.PrivateKey
}

func (a *Account) PubKey() *crypto.PubKey {
	return a.PublicKey
}
