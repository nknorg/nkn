package chain

import (
	"sort"

	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/transaction"
)

const (
	MaxCollectableEntityNum = 20480
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetAllTransactionLists() map[common.Uint160][]*transaction.Transaction
	GetTxnByCount(num int) (map[common.Uint256]*transaction.Transaction, error)
	GetTransaction(hash common.Uint256) *transaction.Transaction
	AppendTxnPool(txn *transaction.Transaction) error
	CleanSubmittedTransactions(txns []*transaction.Transaction) error
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

func (tc *TxnCollector) Collect() (*TxnCollection, error) {
	txnLists := tc.TxnSource.GetAllTransactionLists()
	collection := NewTxnCollection(txnLists)

	return collection, nil
}

func (tc *TxnCollector) GetTransaction(hash common.Uint256) *transaction.Transaction {
	return tc.TxnSource.GetTransaction(hash)
}

func (tc *TxnCollector) Cleanup(txns []*transaction.Transaction) error {
	return tc.TxnSource.CleanSubmittedTransactions(txns)
}

type TxnCollection struct {
	txns map[common.Uint160][]*transaction.Transaction
	tops []*transaction.Transaction
}

func NewTxnCollection(txnLists map[common.Uint160][]*transaction.Transaction) *TxnCollection {
	tops := make([]*transaction.Transaction, 0)
	for addr, txnList := range txnLists {
		tops = append(tops, txnList[0])
		txnLists[addr] = txnList[1:]
	}

	sort.Sort(sort.Reverse(transaction.DefaultSort(tops)))

	return &TxnCollection{
		txns: txnLists,
		tops: tops,
	}
}

func (tc *TxnCollection) Peek() *transaction.Transaction {
	if len(tc.tops) == 0 {
		return nil
	}
	return tc.tops[0]
}

func (tc *TxnCollection) Update() error {
	hashes, err := tc.tops[0].GetProgramHashes()
	if err != nil {
		return err
	}
	if txnList, ok := tc.txns[hashes[0]]; ok && len(txnList) > 0 {
		tc.tops[0], tc.txns[hashes[0]] = txnList[0], txnList[1:]
		sort.Sort(sort.Reverse(transaction.DefaultSort(tc.tops)))
	} else {
		tc.tops = tc.tops[1:]
	}

	return nil
}

func (tc *TxnCollection) Pop() *transaction.Transaction {
	if len(tc.tops) == 0 {
		return nil
	}
	top := tc.tops[0]
	tc.tops = tc.tops[1:]
	return top
}
