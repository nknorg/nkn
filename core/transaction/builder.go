package transaction

import (
	"math/rand"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/types"
)

const (
	TransactionNonceLength = 32
)

func NewTransferAssetTransaction(sender, recipient Uint160, value, fee Fixed64) (*Transaction, error) {
	payload := types.NewTransferAsset(sender, recipient, value)
	pl, err := types.Pack(types.TransferAssetType, payload)
	if err != nil {
		return nil, err
	}

	tx := types.NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewCommitTransaction(sigChain []byte, submitter Uint160) (*Transaction, error) {
	payload := types.NewCommit(sigChain, submitter)
	pl, err := types.Pack(types.CommitType, payload)
	if err != nil {
		return nil, err
	}

	tx := types.NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewRegisterNameTransaction(registrant []byte, name string) (*Transaction, error) {
	payload := types.NewRegisterName(registrant, name)
	pl, err := types.Pack(types.RegisterNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := types.NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}

func NewDeleteNameTransaction(registrant []byte) (*Transaction, error) {
	payload := types.NewDeleteName(registrant)
	pl, err := types.Pack(types.DeleteNameType, payload)
	if err != nil {
		return nil, err
	}

	tx := types.NewMsgTx(pl, rand.Uint64(), 0, util.RandomBytes(TransactionNonceLength))
	return &Transaction{
		MsgTx: *tx,
	}, nil
}
