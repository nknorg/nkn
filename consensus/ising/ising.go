package ising

import (
	"nkn/wallet"
	"nkn/net"
)

func StartIsingConsensus(account *wallet.Account, node net.Neter) {
	proposerService := NewProposerService(account, node)
	go proposerService.Start()
	voterService := NewVoterService(account, node)
	go voterService.Start()
}
