package ising

import (
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/wallet"
)

type ConsensusService struct {
	proposerService *ProposerService
	probeService    *ProbeService
}

func NewConsensusService(account *wallet.Account, node protocol.Noder, porServer *por.PorServer) *ConsensusService {
	consensusService := &ConsensusService{
		proposerService: NewProposerService(account, node, porServer),
		probeService:    NewProbeService(account, node),
	}

	return consensusService
}

func (cs *ConsensusService) Start() {
	go cs.proposerService.Start()
	go cs.probeService.Start()

	for {
		select {
		case msg := <-cs.probeService.msgChan:
			cs.proposerService.msgChan <- msg
		}
	}
}

func StartIsingConsensus(account *wallet.Account, node protocol.Noder, porServer *por.PorServer) {
	service := NewConsensusService(account, node, porServer)
	service.Start()
}
