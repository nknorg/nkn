package vault

import (
	"fmt"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/program"
)

type Account struct {
	PrivateKey  []byte
	PublicKey   []byte
	ProgramHash Uint160
}

func NewAccount() (*Account, error) {
	publicKey, privateKey, err := crypto.GenKeyPair()
	if err != nil {
		return nil, err
	}

	programHash, err := program.CreateProgramHash(publicKey)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, "New account redeemhash generated failed")
	}

	account := &Account{
		PrivateKey:  privateKey,
		PublicKey:   publicKey,
		ProgramHash: programHash,
	}

	return account, nil
}

func NewAccountWithSeed(seed []byte) (*Account, error) {
	if err := crypto.CheckSeed(seed); err != nil {
		return nil, err
	}

	privateKey := crypto.GetPrivateKeyFromSeed(seed)
	pubKey := crypto.GetPublicKeyFromPrivateKey(privateKey)
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

func (a *Account) PubKey() []byte {
	return a.PublicKey
}
