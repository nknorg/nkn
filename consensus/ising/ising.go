package ising

import (
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/vault"
)

type ConsensusService struct {
	proposerService *ProposerService
	probeService    *ProbeService
}

func NewConsensusService(account *vault.Account, node protocol.Noder) *ConsensusService {
	consensusService := &ConsensusService{
		proposerService: NewProposerService(account, node),
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

func StartIsingConsensus(account *vault.Account, node protocol.Noder) {
	service := NewConsensusService(account, node)
	service.Start()
}
