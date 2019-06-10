package transaction

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/pb"
)

const (
	TransactionNonceLength = 32
)

func NewTransferAssetTransaction(sender, recipient Uint160, nonce uint64, value, fee Fixed64) (*Transaction, error) {
	payload := NewTransferAsset(sender, recipient, value)
	pl, err := Pack(pb.TransferAssetType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewCommitTransaction(sigChain []byte, submitter Uint160, nonce uint64) (*Transaction, error) {
	payload := NewCommit(sigChain, submitter)
	pl, err := Pack(pb.CommitType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewRegisterNameTransaction(registrant []byte, name string, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewRegisterName(registrant, name)
	pl, err := Pack(pb.RegisterNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewDeleteNameTransaction(registrant []byte, name string, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewDeleteName(registrant, name)
	pl, err := Pack(pb.DeleteNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, bucket uint32, duration uint32, meta string, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewSubscribe(subscriber, identifier, topic, bucket, duration, meta)
	pl, err := Pack(pb.SubscribeType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewGenerateIDTransaction(publicKey []byte, regFee Fixed64, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewGenerateID(publicKey, regFee)
	pl, err := Pack(pb.GenerateIDType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewNanoPayTransaction(sender, recipient Uint160, nonce uint64, amount Fixed64, height, duration uint32) (*Transaction, error) {
	payload := NewNanoPay(sender, recipient, nonce, amount, height, duration)
	pl, err := Pack(pb.NanoPayType, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, 0, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}
