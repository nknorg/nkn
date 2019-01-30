package types

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
)

const (
	TransactionNonceLength = 32
)

func NewTransferAssetTransaction(sender, recipient Uint160, nonce uint64, value, fee Fixed64) (*Transaction, error) {
	payload := NewTransferAsset(sender, recipient, value)
	pl, err := Pack(TransferAssetType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewCommitTransaction(sigChain []byte, submitter Uint160, nonce uint64) (*Transaction, error) {
	payload := NewCommit(sigChain, submitter)
	pl, err := Pack(CommitType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewRegisterNameTransaction(registrant []byte, name string, nonce uint64) (*Transaction, error) {
	payload := NewRegisterName(registrant, name)
	pl, err := Pack(RegisterNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewDeleteNameTransaction(registrant []byte, nonce uint64) (*Transaction, error) {
	payload := NewDeleteName(registrant)
	pl, err := Pack(DeleteNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}
