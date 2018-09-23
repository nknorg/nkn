package net

import (
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/net/chord"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/net/protocol"
)

func StartProtocol(pubKey *crypto.PubKey, ring *chord.Ring) protocol.Noder {
	net := node.InitNode(pubKey, ring)
	net.ConnectNeighbors()

	return net
}
