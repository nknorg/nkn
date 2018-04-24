package ising

import (
	"fmt"
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	ProbeFactor        = 2
	ResultFactor       = 3
	ProbeDuration      = ConsensusTime / ProbeFactor
	WaitForProbeResult = ConsensusTime / ResultFactor
)

type ProbeService struct {
	sync.RWMutex
	account              *wallet.Account          // local account
	localNode            protocol.Noder           // local node
	ticker               *time.Ticker             // ticker for probing
	detectedResults      map[uint64]StateResponse // collected probe response
	consensusMsgReceived events.Subscriber        // consensus events listening
	msgChan              chan interface{}         // send probe message
}

func NewProbeService(account *wallet.Account, node protocol.Noder, ch chan interface{}) *ProbeService {
	service := &ProbeService{
		account:         account,
		localNode:       node,
		ticker:          time.NewTicker(ProbeDuration),
		detectedResults: make(map[uint64]StateResponse),
		msgChan:         ch,
	}

	return service
}

func (p *ProbeService) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ReceiveConsensusMsg)
	for {
		select {
		case <-p.ticker.C:
			stateProbe := &StateProbe{
				message: "Hi",
			}
			p.SendConsensusMsg(stateProbe)
			time.Sleep(WaitForProbeResult)
			p.AnalyzeResponse()
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
		switch t := isingMsg.(type) {
		case *StateResponse:
			p.HandleStateResponseMsg(t, sender)
		}
	}
}

func (p *ProbeService) SendConsensusMsg(msg IsingMessage) error {
	isingPld, err := BuildIsingPayload(msg, p.account.PublicKey)
	if err != nil {
		return err
	}
	hash, err := isingPld.DataHash()
	if err != nil {
		return err
	}
	signature, err := crypto.Sign(p.account.PrivateKey, hash)
	if err != nil {
		return err
	}
	isingPld.Signature = signature

	// broadcast consensus message
	err = p.localNode.Xmit(isingPld)
	if err != nil {
		return err
	}

	return nil
}

func (p *ProbeService) HandleStateResponseMsg(msg *StateResponse, sender *crypto.PubKey) {
	p.Lock()
	defer p.Unlock()

	id := publickKeyToNodeID(sender)
	p.detectedResults[id] = *msg

	return
}

func (p *ProbeService) AnalyzeResponse() {
	p.RLock()
	defer p.RUnlock()

	results := make(map[Uint256]int)
	for _, resp := range p.detectedResults {
		results[resp.currentBlockHash]++
	}
	length := len(p.detectedResults)
	for blockHash, num := range results {
		if num*2 >= length {
			notice := &BlockInfoNotice{
				hash: blockHash,
			}
			p.msgChan <- notice
			return
		}
	}
	log.Warn("inconsistent state of neighbor")
	return
}
