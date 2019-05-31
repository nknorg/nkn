package pool

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/transaction"
)

type TxPooler interface {
	GetTxnByCount(num int) (map[Uint256]*transaction.Transaction, error)
	GetTransaction(hash Uint256) *transaction.Transaction
	AppendTxnPool(txn *transaction.Transaction) error
	CleanSubmittedTransactions(txns []*transaction.Transaction) error
}
