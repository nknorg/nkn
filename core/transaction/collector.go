package transaction

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/errors"
)

const (
	MaxCollectableEntityNum = 20480
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetTxnByCount(num int, hash Uint256) (map[Uint256]*Transaction, error)
	GetTransaction(hash Uint256) *Transaction
	AppendTxnPool(txn *Transaction) errors.ErrCode
	CleanSubmittedTransactions(txns []*Transaction) error
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

func (tc *TxnCollector) Collect(winningHash Uint256) (map[Uint256]*Transaction, error) {
	return tc.TxnSource.GetTxnByCount(tc.TxnNum, winningHash)
}

func (tc *TxnCollector) GetTransaction(hash Uint256) *Transaction {
	return tc.TxnSource.GetTransaction(hash)
}

func (tc *TxnCollector) Append(txn *Transaction) errors.ErrCode {
	return tc.TxnSource.AppendTxnPool(txn)
}

func (tc *TxnCollector) Cleanup(txns []*Transaction) error {
	return tc.TxnSource.CleanSubmittedTransactions(txns)
}
