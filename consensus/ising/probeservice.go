package ising

import (
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/wallet"
)

const (
	ProbeFactor        = 1
	ResultFactor       = 3
	ProbeDuration      = ConsensusTime / ProbeFactor
	WaitForProbeResult = ConsensusTime / ResultFactor

	// probe service will detect BlockNumDetected blocks of neighbor per time
	BlockNumDetected = 5

	// block detecting starts from height (current height - BlockDepthDetected)
	BlockDepthDetected = 10
)

type ProbeService struct {
	sync.RWMutex
	account              *wallet.Account          // local account
	localNode            protocol.Noder           // local node
	ticker               *time.Ticker             // ticker for probing
	neighborInfo         map[uint64]StateResponse // collected neighbor block info
	startHeight          uint32                   // block start height to be detected
	consensusMsgReceived events.Subscriber        // consensus events listening
	msgChan              chan interface{}         // send probe message
}

func NewProbeService(account *wallet.Account, node protocol.Noder, ch chan interface{}) *ProbeService {
	service := &ProbeService{
		account:      account,
		localNode:    node,
		ticker:       time.NewTicker(ProbeDuration),
		neighborInfo: make(map[uint64]StateResponse),
		startHeight:  0,
		msgChan:      ch,
	}

	return service
}

func (p *ProbeService) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ReceiveConsensusMsg)
	for {
		select {
		case <-p.ticker.C:
			if ledger.DefaultLedger.Store.GetHeight() > BlockDepthDetected {
				stateProbe := &StateProbe{
					ProbeType: BlockHistory,
					ProbePayload: &BlockHistoryPayload{
						p.startHeight,
						p.startHeight + BlockNumDetected,
					},
				}
				p.SendConsensusMsg(stateProbe)
				time.Sleep(WaitForProbeResult)
				p.msgChan <- p.BuildNotice()
				p.Reset()
			}
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
	p.neighborInfo[id] = *msg

	return
}

func (p *ProbeService) BuildNotice() *Notice {
	p.RLock()
	defer p.RUnlock()

	blockHistory := make(map[string]uint64)
	type BlockInfo struct {
		count      int    // count of block
		neighborID uint64 // from which neighbor
	}
	i := p.startHeight
	for i < p.startHeight+BlockNumDetected {
		m := make(map[string]*BlockInfo)
		for id, resp := range p.neighborInfo {
			if hash, ok := resp.PersistedBlocks[i]; ok {
				tmp := HHToString(i, hash)
				if _, ok := m[tmp]; !ok {
					info := &BlockInfo{
						count:      1,
						neighborID: id,
					}
					m[tmp] = info
				} else {
					m[tmp].count++
				}
			}
		}
		// requiring >= 50% neighbors have same block
		for k, v := range m {
			if v.count*2 >= len(p.neighborInfo) {
				if _, ok := blockHistory[k]; !ok {
					blockHistory[k] = v.neighborID
				}
			}
		}
		i++
	}

	return &Notice{
		BlockHistory: blockHistory,
	}
}

func (p *ProbeService) Reset() {
	p.neighborInfo = nil
	p.neighborInfo = make(map[uint64]StateResponse)
	p.startHeight = 0
	if height := ledger.DefaultLedger.Store.GetHeight() - BlockDepthDetected; height > 0 {
		p.startHeight = height
	}
}
