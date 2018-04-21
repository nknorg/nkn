package net

import (
	"time"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/net/protocol"
)

const (
	// waiting for neighbors to connect
	WaitingForOtherNodes = 10 * time.Second
)

func StartProtocol(pubKey *crypto.PubKey) protocol.Noder {
	net := node.InitNode(pubKey)
	net.ConnectSeeds()
	time.Sleep(WaitingForOtherNodes)

	return net
}