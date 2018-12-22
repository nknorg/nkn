package net

import (
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet"
)

func StartProtocol(account *vault.Account, nn *nnet.NNet) (protocol.Noder, error) {
	return node.InitNode(account, nn)
}
