package ising

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/wallet"
)

// chan messaage sent from probe to proposer
type BlockInfoNotice struct {
	hash Uint256
}

func StartIsingConsensus(account *wallet.Account, node protocol.Noder) {
	// TODO communication among goroutines
	msgChan := make(chan interface{}, 1)
	proposerService := NewProposerService(account, node, msgChan)
	go proposerService.Start()
	voterService := NewVoterService(account, node)
	go voterService.Start()
	probeService := NewProbeService(account, node, msgChan)
	go probeService.Start()
}
