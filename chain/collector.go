package chain

import (
	"sort"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/transaction"
)

const (
	MaxCollectableEntityNum = 20480
)

// Transaction pool should be a concrete entity of this interface
type TxnSource interface {
	GetAllTransactionLists() map[Uint160][]*transaction.Transaction
	GetTxnByCount(num int) (map[Uint256]*transaction.Transaction, error)
	GetTransaction(hash Uint256) *transaction.Transaction
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

func (tc *TxnCollector) GetTransaction(hash Uint256) *transaction.Transaction {
	return tc.TxnSource.GetTransaction(hash)
}

func (tc *TxnCollector) Cleanup(txns []*transaction.Transaction) error {
	return tc.TxnSource.CleanSubmittedTransactions(txns)
}

type sortTxnsByPrice []*transaction.Transaction

func (s sortTxnsByPrice) Len() int           { return len(s) }
func (s sortTxnsByPrice) Less(i, j int) bool { return s[i].UnsignedTx.Fee > s[j].UnsignedTx.Fee }
func (s sortTxnsByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type TxnCollection struct {
	txns map[Uint160][]*transaction.Transaction
	tops []*transaction.Transaction
}

func NewTxnCollection(txnLists map[Uint160][]*transaction.Transaction) *TxnCollection {
	tops := make([]*transaction.Transaction, 0)
	for addr, txnList := range txnLists {
		tops = append(tops, txnList[0])
		txnLists[addr] = txnList[1:]
	}

	sort.Sort(sort.Reverse(sortTxnsByPrice(tops)))

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
		sort.Sort(sort.Reverse(sortTxnsByPrice(tc.tops)))
	} else {
		tc.tops = tc.tops[1:]
	}

	return nil
}

func (tc *TxnCollection) Pop() {
	if len(tc.tops) > 0 {
		tc.tops = tc.tops[1:]
	}
}
