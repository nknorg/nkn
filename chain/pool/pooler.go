package pool

import (
	. "github.com/nknorg/nkn/common"
	. "github.com/nknorg/nkn/transaction"
)

type TxPooler interface {
	GetTxnByCount(num int) (map[Uint256]*Transaction, error)
	GetTransaction(hash Uint256) *Transaction
	AppendTxnPool(txn *Transaction) error
	CleanSubmittedTransactions(txns []*Transaction) error
}
