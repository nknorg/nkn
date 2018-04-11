package net

import (
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/net/protocol"
)

func StartProtocol(pubKey *crypto.PubKey) protocol.Noder {
	net := node.InitNode(pubKey)
	net.ConnectSeeds()

	return net
}
