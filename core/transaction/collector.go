
package transaction

const (
	MaxCollectableEntityNum = 1024
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetTxn(num int) []*Transaction
}

// TxnCollector collects transactions from transaction pool
type TxnCollector struct {
	TxnNum    int
	TxnSource TxnSource
}

func New(source TxnSource, num int) *TxnCollector {
	var entityNum int
	if num < 0 {
		entityNum = 0
	} else if num > MaxCollectableEntityNum {
		entityNum = MaxCollectableEntityNum
	}
	return &TxnCollector{
		TxnNum:    entityNum,
		TxnSource: source,
	}
}

func (p *TxnCollector) Collect() []*Transaction {
	return p.TxnSource.GetTxn(p.TxnNum)
}
