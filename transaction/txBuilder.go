package transaction

import (
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util"
)

const (
	TransactionNonceLength = 32
)

func NewTransferAssetTransaction(sender, recipient common.Uint160, nonce uint64, value, fee common.Fixed64) (*Transaction, error) {
	payload := NewTransferAsset(sender, recipient, value)
	pl, err := Pack(pb.PayloadType_TRANSFER_ASSET_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewSigChainTransaction(sigChain []byte, submitter common.Uint160, nonce uint64) (*Transaction, error) {
	payload := NewSigChainTxn(sigChain, submitter)
	pl, err := Pack(pb.PayloadType_SIG_CHAIN_TXN_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewRegisterNameTransaction(registrant []byte, name string, nonce uint64, regFee common.Fixed64, fee common.Fixed64) (*Transaction, error) {
	payload := NewRegisterName(registrant, name, int64(regFee))
	pl, err := Pack(pb.PayloadType_REGISTER_NAME_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewTransferNameTransaction(registrant []byte, to []byte, name string, nonce uint64, fee common.Fixed64) (*Transaction, error) {
	payload := NewTransferName(registrant, to, name)
	pl, err := Pack(pb.PayloadType_TRANSFER_NAME_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewDeleteNameTransaction(registrant []byte, name string, nonce uint64, fee common.Fixed64) (*Transaction, error) {
	payload := NewDeleteName(registrant, name)
	pl, err := Pack(pb.PayloadType_DELETE_NAME_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewSubscribeTransaction(subscriber []byte, identifier string, topic string, duration uint32, meta string, nonce uint64, fee common.Fixed64) (*Transaction, error) {
	payload := NewSubscribe(subscriber, identifier, topic, duration, meta)
	pl, err := Pack(pb.PayloadType_SUBSCRIBE_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewUnsubscribeTransaction(subscriber []byte, identifier string, topic string, nonce uint64, fee common.Fixed64) (*Transaction, error) {
	payload := NewUnsubscribe(subscriber, identifier, topic)
	pl, err := Pack(pb.PayloadType_UNSUBSCRIBE_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewGenerateIDTransaction(publicKey, sender []byte, regFee common.Fixed64, version int32, nonce uint64, fee common.Fixed64, attrs []byte) (*Transaction, error) {
	payload := NewGenerateID(publicKey, sender, regFee, version)
	pl, err := Pack(pb.PayloadType_GENERATE_ID_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, attrs)

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewNanoPayTransaction(sender, recipient common.Uint160, id uint64, amount common.Fixed64, txnExpiration, nanoPayExpiration uint32) (*Transaction, error) {
	payload := NewNanoPay(sender, recipient, id, amount, txnExpiration, nanoPayExpiration)
	pl, err := Pack(pb.PayloadType_NANO_PAY_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, 0, 0, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}

func NewIssueAssetTransaction(sender common.Uint160, name, symbol string, totalSupply common.Fixed64, precision uint32, nonce uint64, fee common.Fixed64) (*Transaction, error) {
	payload := NewIssueAsset(sender, name, symbol, precision, totalSupply)
	pl, err := Pack(pb.PayloadType_ISSUE_ASSET_TYPE, payload)
	if err != nil {
		return nil, err
	}

	tx := NewMsgTx(pl, nonce, fee, util.RandomBytes(TransactionNonceLength))

	return &Transaction{
		Transaction: tx,
	}, nil
}
