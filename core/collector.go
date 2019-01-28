package core

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/types"
)

const (
	MaxCollectableEntityNum = 20480
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetTxnByCount(num int) (map[Uint256]*types.Transaction, error)
	GetTransaction(hash Uint256) *types.Transaction
	AppendTxnPool(txn *types.Transaction) errors.ErrCode
	CleanSubmittedTransactions(txns []*types.Transaction) error
}

// TxnCollector collects transactions from transaction pool
type TxnCollector struct {
	TxnNum    int
	TxnSource TxnSource
}

func NewTxnCollector(source TxnSource, num int) *TxnCollector {
	var entityNum int
	if num <= 0 {
		entityNum = 0
	} else if num > MaxCollectableEntityNum {
		entityNum = MaxCollectableEntityNum
	} else {
		entityNum = num
	}

	return &TxnCollector{
		TxnNum:    entityNum,
		TxnSource: source,
	}
}

func (tc *TxnCollector) Collect() (map[Uint256]*types.Transaction, error) {
	return tc.TxnSource.GetTxnByCount(tc.TxnNum)
}

func (tc *TxnCollector) GetTransaction(hash Uint256) *types.Transaction {
	return tc.TxnSource.GetTransaction(hash)
}

func (tc *TxnCollector) Append(txn *types.Transaction) errors.ErrCode {
	return tc.TxnSource.AppendTxnPool(txn)
}

func (tc *TxnCollector) Cleanup(txns []*types.Transaction) error {
	return tc.TxnSource.CleanSubmittedTransactions(txns)
}
