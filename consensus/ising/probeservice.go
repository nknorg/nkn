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
	"github.com/nknorg/nkn/vault"
)

const (
	ProbeFactor        = 1
	ResultFactor       = 3
	ProbeDuration      = ConsensusTime / ProbeFactor
	WaitForProbeResult = ConsensusTime / ResultFactor
	MsgChanCap         = 1

	// probe service will detect BlockNumDetected blocks of neighbor per time
	BlockNumDetected = 5

	// block detecting starts from height (current height - BlockDepthDetected)
	BlockDepthDetected = 10
)

// chan messaage sent from probe to proposer
type Notice struct {
	BlockHistory map[string]uint64
}

type ProbeService struct {
	sync.RWMutex
	account              *vault.Account           // local account
	localNode            protocol.Noder           // local node
	ticker               *time.Ticker             // ticker for probing
	neighborInfo         map[uint64]StateResponse // collected neighbor block info
	startHeight          uint32                   // block start height to be detected
	consensusMsgReceived events.Subscriber        // consensus events listening
	msgChan              chan interface{}         // send probe message
}

func NewProbeService(account *vault.Account, node protocol.Noder) *ProbeService {
	service := &ProbeService{
		account:      account,
		localNode:    node,
		ticker:       time.NewTicker(ProbeDuration),
		neighborInfo: make(map[uint64]StateResponse),
		startHeight:  0,
		msgChan:      make(chan interface{}, MsgChanCap),
	}

	return service
}

func (ps *ProbeService) Start() error {
	ps.consensusMsgReceived = ps.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, ps.ReceiveConsensusMsg)
	for {
		select {
		case <-ps.ticker.C:
			if ledger.DefaultLedger.Store.GetHeight() > BlockDepthDetected {
				stateProbe := &StateProbe{
					ProbeType: BlockHistory,
					ProbePayload: &BlockHistoryPayload{
						ps.startHeight,
						ps.startHeight + BlockNumDetected,
					},
				}
				ps.SendConsensusMsg(stateProbe)
				time.Sleep(WaitForProbeResult)
				ps.msgChan <- ps.BuildNotice()
				ps.Reset()
			}
		}
	}

	return nil
}

func (ps *ProbeService) ReceiveConsensusMsg(v interface{}) {
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
			ps.HandleStateResponseMsg(t, sender)
		}
	}
}

func (ps *ProbeService) SendConsensusMsg(msg IsingMessage) error {
	isingPld, err := BuildIsingPayload(msg, ps.account.PublicKey)
	if err != nil {
		return err
	}
	hash, err := isingPld.DataHash()
	if err != nil {
		return err
	}
	signature, err := crypto.Sign(ps.account.PrivateKey, hash)
	if err != nil {
		return err
	}
	isingPld.Signature = signature

	// broadcast consensus message
	err = ps.localNode.Xmit(isingPld)
	if err != nil {
		return err
	}

	return nil
}

func (ps *ProbeService) HandleStateResponseMsg(msg *StateResponse, sender *crypto.PubKey) {
	ps.Lock()
	defer ps.Unlock()

	id := publickKeyToNodeID(sender)
	ps.neighborInfo[id] = *msg

	return
}

func (ps *ProbeService) BuildNotice() *Notice {
	ps.RLock()
	defer ps.RUnlock()

	blockHistory := make(map[string]uint64)
	type BlockInfo struct {
		count      int    // count of block
		neighborID uint64 // from which neighbor
	}
	i := ps.startHeight
	for i < ps.startHeight+BlockNumDetected {
		m := make(map[string]*BlockInfo)
		for id, resp := range ps.neighborInfo {
			if hash, ok := resp.PersistedBlocks[i]; ok {
				tmp := HeightHashToString(i, hash)
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
			if v.count*2 >= len(ps.neighborInfo) {
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

func (ps *ProbeService) Reset() {
	ps.neighborInfo = nil
	ps.neighborInfo = make(map[uint64]StateResponse)
	ps.startHeight = 0
	if height := ledger.DefaultLedger.Store.GetHeight() - BlockDepthDetected; height > 0 {
		ps.startHeight = height
	}
}
