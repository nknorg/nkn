package vault

import (
	"fmt"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/vm/contract"
)

type Account struct {
	PrivateKey  []byte
	PublicKey   *crypto.PubKey
	ProgramHash Uint160
}

func NewAccount() (*Account, error) {
	priKey, pubKey, _ := crypto.GenKeyPair()

	redeemHash, err := contract.CreateRedeemHash(&pubKey)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, "New account redeemhash generated failed")
	}
	return &Account{
		PrivateKey:  priKey,
		PublicKey:   &pubKey,
		ProgramHash: redeemHash,
	}, nil
}

func NewAccountWithPrivatekey(privateKey []byte) (*Account, error) {
	//privKeyLen := len(privateKey)
	//if privKeyLen != 32 {
	//	return nil, errors.New("invalid private key length")
	//}
	pubKey := crypto.NewPubKey(privateKey)
	redeemHash, err := contract.CreateRedeemHash(pubKey)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, "New account redeemhash generated failed")
	}

	return &Account{
		PrivateKey:  privateKey,
		PublicKey:   pubKey,
		ProgramHash: redeemHash,
	}, nil
}

func (a *Account) PrivKey() []byte {
	return a.PrivateKey
}

func (a *Account) PubKey() *crypto.PubKey {
	return a.PublicKey
}
