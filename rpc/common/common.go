package common

import (
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/errors"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/vault"
)

type Serverer interface {
	GetNetNode() (protocol.Noder, error)
	GetWallet() (vault.Wallet, error)
	VerifyAndSendTx(txn *transaction.Transaction) errors.ErrCode
}
