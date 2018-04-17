package por

import (
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
)

type IPor interface {
	CalcRelays(txn *transaction.Transaction) error
	GetRelays(pk *crypto.PubKey) int
	GetMaxRelay() crypto.PubKey
	MergeRelays(por IPor) error
	IsRelaying(pk *crypto.PubKey) bool
	TotalRelays() int
	IsTxProcessed(txn *transaction.Transaction) bool
}
