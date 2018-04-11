package net

import (
	"nkn/crypto"
	"nkn/net/node"
	"nkn/net/protocol"
)

func StartProtocol(pubKey *crypto.PubKey) protocol.Noder {
	net := node.InitNode(pubKey)
	net.ConnectSeeds()

	return net
}
