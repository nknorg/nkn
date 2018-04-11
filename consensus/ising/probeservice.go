package ising

import (
	"fmt"
	"time"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/wallet"
)

const (
	ProbeDuration = time.Second
)

type ProbeService struct {
	account              *wallet.Account   // local account
	localNode            protocol.Noder    // local node
	ticker               *time.Ticker      // ticker for probing
	consensusMsgReceived events.Subscriber // consensus events listening
}

func NewProbeService(account *wallet.Account, node protocol.Noder) *ProbeService {
	service := &ProbeService{
		account:   account,
		localNode: node,
		ticker:    time.NewTicker(ProbeDuration),
	}

	return service
}

func (p *ProbeService) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ReceiveConsensusMsg)
	for {
		select {
		case <-p.ticker.C:
			// TODO send State Probe message
		}
	}

	return nil
}

func (p *ProbeService) ReceiveConsensusMsg(v interface{}) {
	if payload, ok := v.(*message.IsingPayload); ok {
		sender := payload.Sender
		signature := payload.Signature
		hash, err := payload.DataHash()
		if err != nil {
			fmt.Println("get consensus payload hash error")
			return
		}
		err = crypto.Verify(*sender, hash, signature)
		if err != nil {
			fmt.Println("consensus message verification error")
			return
		}
		isingMsg, err := RecoverFromIsingPayload(payload)
		if err != nil {
			fmt.Println("Deserialization of ising message error")
			return
		}
		switch isingMsg.(type) {
		//TODO handle Probe response message
		}
	}
}
