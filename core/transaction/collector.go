package transaction

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/errors"
)

const (
	MaxCollectableEntityNum = 1024
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetTxnByCount(num int) map[Uint256]*Transaction
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

func (p *TxnCollector) Collect() map[Uint256]*Transaction {
	return p.TxnSource.GetTxnByCount(p.TxnNum)
}

func (p *TxnCollector) GetTransaction(hash Uint256) *Transaction {
	return p.TxnSource.GetTransaction(hash)
}

func (p *TxnCollector) Append(txn *Transaction) errors.ErrCode {
	return p.TxnSource.AppendTxnPool(txn)
}

func (p *TxnCollector) Cleanup(txns []*Transaction) error {
	return p.TxnSource.CleanSubmittedTransactions(txns)
}
