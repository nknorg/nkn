package por

import (
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
)

type IPor interface {
	CalcRelays(txn *transaction.Transaction) error
	GetRelays(pk *crypto.PubKey) int
	GetMaxRelay() crypto.PubKey
}
