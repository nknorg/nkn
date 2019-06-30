package vault

import (
	"fmt"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/program"
)

type Account struct {
	PrivateKey  []byte
	PublicKey   *crypto.PubKey
	ProgramHash Uint160
}

func NewAccount() (*Account, error) {
	priKey, pubKey, _ := crypto.GenKeyPair()

	programHash, err := program.CreateProgramHash(&pubKey)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, "New account redeemhash generated failed")
	}
	return &Account{
		PrivateKey:  priKey,
		PublicKey:   &pubKey,
		ProgramHash: programHash,
	}, nil
}

func NewAccountWithPrivatekey(privateKey []byte) (*Account, error) {
	pubKey := crypto.NewPubKey(privateKey)
	programHash, err := program.CreateProgramHash(pubKey)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, "New account redeemhash generated failed")
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
