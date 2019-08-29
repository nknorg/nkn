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
	pl, err := Pack(pb.TRANSFER_ASSET_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewSigChainTransaction(sigChain []byte, submitter Uint160, nonce uint64) (*Transaction, error) {
	payload := NewSigChainTxn(sigChain, submitter)
	pl, err := Pack(pb.SIG_CHAIN_TXN_TYPE, payload)
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
	pl, err := Pack(pb.REGISTER_NAME_TYPE, payload)
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
	pl, err := Pack(pb.DELETE_NAME_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, duration uint32, meta string, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewSubscribe(subscriber, identifier, topic, duration, meta)
	pl, err := Pack(pb.SUBSCRIBE_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewUnsubscribeTransaction(subscriber []byte, identifier string, topic string, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewUnsubscribe(subscriber, identifier, topic)
	pl, err := Pack(pb.UNSUBSCRIBE_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewGenerateIDTransaction(publicKey []byte, regFee Fixed64, nonce uint64, fee Fixed64, attrs []byte) (*Transaction, error) {
	payload := NewGenerateID(publicKey, regFee)
	pl, err := Pack(pb.GENERATE_ID_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, attrs)

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewNanoPayTransaction(sender, recipient Uint160, id uint64, amount Fixed64, txnExpiration, nanoPayExpiration uint32) (*Transaction, error) {
	payload := NewNanoPay(sender, recipient, id, amount, txnExpiration, nanoPayExpiration)
	pl, err := Pack(pb.NANO_PAY_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, 0, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewIssueAssetTransaction(sender Uint160, name, symbol string, totalSupply Fixed64, precision uint32, nonce uint64, fee Fixed64) (*Transaction, error) {
	payload := NewIssueAsset(sender, name, symbol, precision, totalSupply)
	pl, err := Pack(pb.ISSUE_ASSET_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}
