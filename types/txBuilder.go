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

func NewDeleteNameTransaction(registrant []byte, name string) (*Transaction, error) {
	payload := NewDeleteName(registrant, name)
	pl, err := Pack(DeleteNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, meta string) (*Transaction, error) {
	payload := NewSubscribe(subscriber, identifier, topic, bucket, duration, meta)
	pl, err := Pack(SubscribeType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		MsgTx: *tx,
	}, nil
}
