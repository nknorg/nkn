package pool

import (
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/transaction"
)

type TxPooler interface {
	GetTxnByCount(num int) (map[common.Uint256]*transaction.Transaction, error)
	GetTransaction(hash common.Uint256) *transaction.Transaction
	AppendTxnPool(txn *transaction.Transaction) error
	CleanSubmittedTransactions(txns []*transaction.Transaction) error
}
