package ising

import (
	"nkn/wallet"
	"nkn/net/protocol"
)

func StartIsingConsensus(account *wallet.Account,  node protocol.Noder) {
	proposerService := NewProposerService(account, node)
	go proposerService.Start()
	voterService := NewVoterService(account, node)
	go voterService.Start()
}
