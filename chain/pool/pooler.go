package pool

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/errors"
	. "github.com/nknorg/nkn/transaction"
)

type TxPooler interface {
	GetTxnByCount(num int, hash Uint256) (map[Uint256]*Transaction, error)
	GetTransaction(hash Uint256) *Transaction
	AppendTxnPool(txn *Transaction) errors.ErrCode
	CleanSubmittedTransactions(txns []*Transaction) error
}
