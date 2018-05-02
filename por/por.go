package por

import (
	"github.com/nknorg/nkn/core/transaction"
)

type IPor interface {
	CalcRelays(txn *transaction.Transaction) error
	GetRelays(pk []byte) int
	GetMaxRelay() []byte
	MergeRelays(por IPor) error
	IsRelaying(pk []byte) bool
	TotalRelays() int
	IsTxProcessed(txn *transaction.Transaction) bool
}
