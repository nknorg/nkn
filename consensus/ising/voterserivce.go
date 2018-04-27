package ising

import (
	"fmt"
	"sync"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

type VoterService struct {
	sync.RWMutex
	account              *wallet.Account    // local account
	state                map[uint64]*State  // consensus state
	localNode            protocol.Noder     // local node
	blockCache           *BlockCache        // blocks waiting for voting
	consensusMsgReceived events.Subscriber  // consensus events listening
	blockChan            chan *ledger.Block // receive block from proposer thread
}

func NewVoterService(account *wallet.Account, node protocol.Noder, bch chan *ledger.Block) *VoterService {
	service := &VoterService{
		account:    account,
		state:      initialVoterNodeState(node),
		localNode:  node,
		blockCache: NewCache(),
		blockChan:  bch,
	}

	// sync new generated block from proposer thread to local cache
	go func() {
		for {
			select {
			case block := <-bch:
				service.blockCache.AddBlockToCache(block)
				for _, s := range service.state {
					s.SetBit(FloodingFinished)
				}
			}
		}
	}()

	return service
}

func (p *VoterService) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ReceiveConsensusMsg)
	return nil
}

func (p *VoterService) ReceiveConsensusMsg(v interface{}) {
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
		case *BlockFlooding:
			p.HandleBlockFloodingMsg(t, sender)
		case *BlockProposal:
			p.HandleBlockProposalMsg(t, sender)
		case *BlockResponse:
			p.HandleBlockResponseMsg(t, sender)
		}
	}
}

func (p *VoterService) NewSenderDetected(node uint64) bool {
	p.RLock()
	defer p.RUnlock()

	if _, ok := p.state[node]; !ok {
		return true
	}

	return false
}

func (p *VoterService) AddNewNeighbor(node uint64) {
	p.Lock()
	defer p.Unlock()

	var s State
	s.SetBit(InitialState)
	p.state[node] = &s

	return
}

func (p *VoterService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender *crypto.PubKey) {
	dumpState(sender, "VoterService received BlockFlooding", 0)
	// TODO check if the sender is PoR node
	err := p.blockCache.AddBlockToCache(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
	for _, v := range p.state {
		v.SetBit(FloodingFinished)
	}
}

func (p *VoterService) HandleBlockProposalMsg(bpMsg *BlockProposal, sender *crypto.PubKey) {
	// TODO check if the sender is neighbor
	nodeID := publickKeyToNodeID(sender)
	if p.NewSenderDetected(nodeID) {
		p.AddNewNeighbor(nodeID)
	}
	dumpState(sender, "VoterService received BlockProposal", *p.state[nodeID])
	if !p.state[nodeID].HasBit(InitialState) || p.state[nodeID].HasBit(OpinionSent) {
		log.Warn("consensus state error in BlockProposal message handler")
		return
	}
	p.Lock()
	defer p.Unlock()
	hash := *bpMsg.blockHash
	if !p.blockCache.BlockInCache(hash) {
		brMsg := &BlockRequest{
			blockHash: bpMsg.blockHash,
		}
		p.ReplyConsensusMsg(brMsg, sender)
		p.state[nodeID].SetBit(RequestSent)
		log.Warn("doesn't contain block in local cache, requesting it from neighbor")
		return
	}

	if !p.state[nodeID].HasBit(FloodingFinished) {
		log.Warn("require FloodingFinished state in BlockProposal message handler")
		return
	}

	block := p.blockCache.GetBlockFromCache(hash)
	if block == nil {
		return
	}
	//TODO verify block
	option := true
	blMsg := &BlockVote{
		blockHash: bpMsg.blockHash,
		agree:     option,
	}
	p.ReplyConsensusMsg(blMsg, sender)
	p.state[nodeID].SetBit(OpinionSent)
	p.Reset(nodeID)
	return
}

func (p *VoterService) HandleBlockResponseMsg(brMsg *BlockResponse, sender *crypto.PubKey) {
	p.Lock()
	defer p.Unlock()

	nodeID := publickKeyToNodeID(sender)
	dumpState(sender, "VoterService received BlockResponse", *p.state[nodeID])
	if !p.state[nodeID].HasBit(InitialState) || !p.state[nodeID].HasBit(RequestSent) {
		log.Warn("consensus state error in BlockResponse message handler")
		return
	}
	// TODO check if the sender is requested neighbor node
	err := p.blockCache.AddBlockToCache(brMsg.block)
	if err != nil {
		return
	}
	p.state[nodeID].SetBit(FloodingFinished)
	// TODO verify block
	option := true
	hash := brMsg.block.Hash()
	bvMsg := &BlockVote{
		blockHash: &hash,
		agree:     option,
	}
	p.ReplyConsensusMsg(bvMsg, sender)
	p.state[nodeID].SetBit(OpinionSent)
	p.Reset(nodeID)
	return
}

func (p *VoterService) ReplyConsensusMsg(msg IsingMessage, to *crypto.PubKey) error {
	replied := false
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
	neighbors := p.localNode.GetNeighborNoder()
	for _, n := range neighbors {
		if n.GetID() == publickKeyToNodeID(to) {
			b, err := message.NewIsingConsensus(isingPld)
			if err != nil {
				return err
			}
			n.Tx(b)
			replied = true
		}
	}
	if !replied {
		log.Warn("neighbor missing")
	}

	return nil
}

// Reset resets neighbor state for continuing next round consensus,
// this function should be called after voter thread sending a vote to a neighbor.
func (p *VoterService) Reset(neighbor uint64) {
	p.state[neighbor].ClearAll()
	p.state[neighbor].SetBit(InitialState)
}

func initialVoterNodeState(node protocol.Noder) map[uint64]*State {
	neighbors := node.GetNeighborNoder()
	m := make(map[uint64]*State, len(neighbors))
	for _, n := range neighbors {
		var state State
		state.SetBit(InitialState)
		m[n.GetID()] = &state
	}

	return m
}
