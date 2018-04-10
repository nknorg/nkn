package ising

import (
	"fmt"
	"nkn/crypto"
	"bytes"
	"encoding/binary"
	"nkn/common/log"
	"nkn/events"
	"nkn/net"
	"nkn/net/message"
	"nkn/wallet"
)

type VoterService struct {
	account              *wallet.Account   // local account
	state                map[uint64]*State // consensus state
	localNode            net.Neter         // local node
	blockCache           *BlockCache       // blocks waiting for voting
	consensusMsgReceived events.Subscriber // consensus events listening
}

func NewVoterService(account *wallet.Account, node net.Neter) *VoterService {
	service := &VoterService{
		account:    account,
		state:      initialVoterNodeState(node),
		localNode:  node,
		blockCache: NewCache(),
	}

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

func (p *VoterService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender *crypto.PubKey) {
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
	hash := *bpMsg.blockHash
	if !p.state[nodeID].HasBit(FloodingFinished) {
		if !p.blockCache.BlockInCache(hash) {
			brMsg := &BlockRequest{
				blockHash: bpMsg.blockHash,
			}
			p.ReplyConsensusMsg(brMsg, sender)
			p.state[nodeID].SetBit(RequestSent)
			return
		}
		p.state[nodeID].SetBit(FloodingFinished)
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
	return
}

func (p *VoterService) HandleBlockResponseMsg(brMsg *BlockResponse, sender *crypto.PubKey) {
	nodeID := publickKeyToNodeID(sender)
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

func publickKeyToNodeID(pubKey *crypto.PubKey) uint64 {
	var id uint64
	key, err := pubKey.EncodePoint(true)
	if err != nil {
		log.Error(err)
	}
	err = binary.Read(bytes.NewBuffer(key[:8]), binary.LittleEndian, &id)
	if err != nil {
		log.Error(err)
	}

	return id
}

func initialVoterNodeState(node net.Neter) map[uint64]*State {
	neighbors := node.GetNeighborNoder()
	m := make(map[uint64]*State, len(neighbors))
	var state State
	state.SetBit(InitialState)
	for _, n := range neighbors {
		m[n.GetID()] = &state
	}

	return m
}