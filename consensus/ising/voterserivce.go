package ising

import (
	"nkn/events"
	"nkn/net"
	"nkn/wallet"
	"time"
)

func InitialVoterNodeState(node net.Neter) map[uint64]State {
	neighbors := node.GetNeighborNoder()
	m := make(map[uint64]State, len(neighbors))
	var state State
	state.SetBit(InitialState)
	for _, n := range neighbors {
		m[n.GetID()] = state
	}

	return m
}

type VoterService struct {
	account              *wallet.Account   // local account
	state                map[uint64]State  // consensus state
	localNode            net.Neter         // local node
	blockCache           *BlockCache       // blocks waiting for voting
	consensusMsgReceived events.Subscriber // consensus events listening
}

func NewVoterService(account *wallet.Account, node net.Neter) *VoterService {
	service := &VoterService{
		account:    account,
		state:      InitialVoterNodeState(node),
		localNode:  node,
		blockCache: NewCache(),
	}

	return service
}

func (p *VoterService) Start() error {
	ticker := time.NewTicker(time.Second * 20)
	for {
		select {
		case <-ticker.C:
			//TODO
		}
	}

	return nil
}