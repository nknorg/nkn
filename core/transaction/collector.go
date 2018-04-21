
package transaction

import (
	. "github.com/nknorg/nkn/common"
)

const (
	MaxCollectableEntityNum = 1024
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetTxnByCount(num int) map[Uint256]*Transaction
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
	}
	return &TxnCollector{
		TxnNum:    entityNum,
		TxnSource: source,
	}
}

func (p *TxnCollector) Collect() map[Uint256]*Transaction {
	return p.TxnSource.GetTxnByCount(p.TxnNum)
}
