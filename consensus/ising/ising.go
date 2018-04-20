package ising

import (
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/wallet"
)

const (
	MsgChanCap   = 1
	BlockChanCap = 1
)

// chan messaage sent from probe to proposer
type BlockInfoNotice struct {
	hash Uint256
}

func StartIsingConsensus(account *wallet.Account, node protocol.Noder) {
	// TODO communication among goroutines
	msgChan := make(chan interface{}, MsgChanCap)
	blockChan := make(chan *ledger.Block, BlockChanCap)
	proposerService := NewProposerService(account, node, msgChan, blockChan)
	go proposerService.Start()
	voterService := NewVoterService(account, node, blockChan)
	go voterService.Start()
	probeService := NewProbeService(account, node, msgChan)
	go probeService.Start()
}
