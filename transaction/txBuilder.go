package transaction

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
	. "github.com/nknorg/nkn/pb"
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

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

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

func NewDeleteNameTransaction(registrant []byte, name string, nonce uint64) (*Transaction, error) {
	payload := NewDeleteName(registrant, name)
	pl, err := Pack(DeleteNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, meta string, nonce uint64) (*Transaction, error) {
	payload := NewSubscribe(subscriber, identifier, topic, bucket, duration, meta)
	pl, err := Pack(SubscribeType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		MsgTx: *tx,
	}, nil
}
