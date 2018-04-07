package net

import (
	. "nkn/common"
	"nkn/core/transaction"
	"nkn/crypto"
	. "nkn/errors"
	"nkn/events"
	"nkn/net/node"
	"nkn/net/protocol"
)

type Neter interface {
	GetTxnByCount(int) map[Uint256]*transaction.Transaction
	GetTxnPool() *transaction.TXNPool
	Xmit(interface{}) error
	GetEvent(eventName string) *events.Event
	GetBookKeepersAddrs() ([]*crypto.PubKey, uint64)
	CleanSubmittedTransactions([]*transaction.Transaction) error
	GetNeighborNoder() []protocol.Noder
	Tx(buf []byte)
	AppendTxnPool(*transaction.Transaction) ErrCode
}

func StartProtocol(pubKey *crypto.PubKey) protocol.Noder {
	net := node.InitNode(pubKey)
	net.ConnectSeeds()

	return net
}
