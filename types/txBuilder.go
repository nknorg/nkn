package types

import (
	"math/rand"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
)

const (
	TransactionNonceLength = 32
)

func NewTransferAssetTransaction(sender, recipient Uint160, value, fee Fixed64) (*Transaction, error) {
	payload := NewTransferAsset(sender, recipient, value)
	pl, err := Pack(TransferAssetType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewCommitTransaction(sigChain []byte, submitter Uint160) (*Transaction, error) {
	payload := NewCommit(sigChain, submitter)
	pl, err := Pack(CommitType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewRegisterNameTransaction(registrant []byte, name string) (*Transaction, error) {
	payload := NewRegisterName(registrant, name)
	pl, err := Pack(RegisterNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewDeleteNameTransaction(registrant []byte) (*Transaction, error) {
	payload := NewDeleteName(registrant)
	pl, err := Pack(DeleteNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}
