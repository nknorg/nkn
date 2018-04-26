package ising

import (
	"fmt"
	"sync"

	. "github.com/nknorg/nkn/common"
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
	account              *wallet.Account               // local account
	state                map[uint64]map[Uint256]*State // consensus state
	localNode            protocol.Noder                // local node
	blockCache           *BlockCache                   // blocks waiting for voting
	consensusMsgReceived events.Subscriber             // consensus events listening
	blockChan            chan *ledger.Block            // receive block from proposer thread
}

func NewVoterService(account *wallet.Account, node protocol.Noder, bch chan *ledger.Block) *VoterService {
	service := &VoterService{
		account:    account,
		state:      make(map[uint64]map[Uint256]*State),
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
				neighbors := node.GetNeighborNoder()
				for _, v := range neighbors {
					id := publickKeyToNodeID(v.GetPubKey())
					service.SetNeighborBlockState(id, block.Hash(), FloodingFinished)
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

	if _, ok := p.state[node]; !ok {
		p.state[node] = make(map[Uint256]*State)
	}

	return
}

func (p *VoterService) HasNeighborBlockState(id uint64, blockhash Uint256, state State) bool {
	if _, ok := p.state[id]; !ok {
		return false
	} else {
		if v, ok := p.state[id][blockhash]; !ok || v == nil {
			return false
		} else {
			if v.HasBit(state) {
				return true
			}
			return false
		}
	}
}

func (p *VoterService) SetNeighborBlockState(id uint64, blockhash Uint256, s State) {
	if _, ok := p.state[id]; !ok {
		p.state[id] = make(map[Uint256]*State)
	}
	if _, ok := p.state[id][blockhash]; !ok {
		p.state[id][blockhash] = new(State)
	}
	p.state[id][blockhash].SetBit(s)
}

func (p *VoterService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender *crypto.PubKey) {
	dumpState(sender, "VoterService received BlockFlooding", nil)
	// TODO check if the sender is PoR node
	err := p.blockCache.AddBlockToCache(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
	neighbors := p.localNode.GetNeighborNoder()
	for _, v := range neighbors {
		id := publickKeyToNodeID(v.GetPubKey())
		p.SetNeighborBlockState(id, bfMsg.block.Hash(), FloodingFinished)
	}
}

func (p *VoterService) HandleBlockProposalMsg(bpMsg *BlockProposal, sender *crypto.PubKey) {
	// TODO check if the sender is neighbor
	nodeID := publickKeyToNodeID(sender)
	if p.NewSenderDetected(nodeID) {
		p.AddNewNeighbor(nodeID)
	}
	hash := *bpMsg.blockHash
	dumpState(sender, "VoterService received BlockProposal", p.state[nodeID][hash])
	if p.HasNeighborBlockState(nodeID, hash, OpinionSent) {
		log.Warn("consensus state error in BlockProposal message handler")
		return
	}
	p.Lock()
	defer p.Unlock()
	if !p.blockCache.BlockInCache(hash) {
		brMsg := &BlockRequest{
			blockHash: bpMsg.blockHash,
		}
		p.ReplyConsensusMsg(brMsg, sender)
		p.SetNeighborBlockState(nodeID, hash, RequestSent)
		log.Warn("doesn't contain block in local cache, requesting it from neighbor")
		return
	}

	if !p.HasNeighborBlockState(nodeID, hash, FloodingFinished) {
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
	p.SetNeighborBlockState(nodeID, hash, OpinionSent)
}

func (p *VoterService) HandleBlockResponseMsg(brMsg *BlockResponse, sender *crypto.PubKey) {
	p.Lock()
	defer p.Unlock()

	hash := brMsg.block.Hash()
	nodeID := publickKeyToNodeID(sender)
	dumpState(sender, "VoterService received BlockResponse", p.state[nodeID][hash])
	if !p.HasNeighborBlockState(nodeID, hash, RequestSent) {
		log.Warn("consensus state error in BlockResponse message handler")
		return
	}
	// TODO check if the sender is requested neighbor node
	err := p.blockCache.AddBlockToCache(brMsg.block)
	if err != nil {
		return
	}
	p.SetNeighborBlockState(nodeID, hash, FloodingFinished)
	// TODO verify block
	option := true
	bvMsg := &BlockVote{
		blockHash: &hash,
		agree:     option,
	}
	p.ReplyConsensusMsg(bvMsg, sender)
	p.SetNeighborBlockState(nodeID, hash, OpinionSent)
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
