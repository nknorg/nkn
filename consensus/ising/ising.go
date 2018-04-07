package ising

import (
	"fmt"
	"math/rand"

	. "nkn/common"
	"nkn/common/log"
	"nkn/common/serialization"
	"nkn/core/contract/program"
	"nkn/core/ledger"
	"nkn/core/transaction"
	"nkn/core/transaction/payload"
	"nkn/crypto"
	"nkn/events"
	"nkn/net"
	"nkn/net/message"
	"nkn/wallet"
)

const (
	TxnAmountToBePackaged = 1024
)

var MsgSignatureStub = [32]byte{}

type Ising struct {
	wallet               wallet.Wallet             // local account
	role                 Bitmap                    // node role
	state                Bitmap                    // consensus state
	localNode            net.Neter                 // local node
	txnCollector         *transaction.TxnCollector // collect transaction from where
	confirmingBlock      *Uint256                  // current block in consensus process
	proposalNum          int32                     // sent BlockProposal msg count
	votedNum             int32                     // received agreed BlockVote msg count
	blockCache           *BlockCache               // blocks waiting for voting
	consensusMsgReceived events.Subscriber         // consensus events listening
}

func New(wallet wallet.Wallet, node net.Neter) *Ising {
	var role Bitmap

	role.SetBit(BlockVoter)
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil
	}
	encPubKey, err := account.PublicKey.EncodePoint(true)
	if err != nil {
		return nil
	}
	confPubKey, err := ledger.StandbyBookKeepers[0].EncodePoint(true)
	if IsEqualBytes(encPubKey, confPubKey) {
		role.SetBit(BlockProposer)
	}

	ising := &Ising{
		role:         role,
		state:        InitialState,
		wallet:       wallet,
		localNode:    node,
		txnCollector: transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged),
		blockCache:   NewCache(),
	}

	return ising
}

func (p *Ising) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ReceiveConsensusMsg)
	if p.role.HasBit(BlockProposer) {
		fmt.Println("I am block proposer")
		block, err := p.BuildBlock()
		if err != nil {
			return err
		}
		err = p.blockCache.AddBlockToCache(block)
		if err != nil {
			return err
		}
		blockFlooding := &BlockFlooding{
			block: block,
		}
		err = p.SendConsensusMsg(blockFlooding)
		if err != nil {
			return err
		}
		p.state.SetBit(FloodingFinished)
	}

	return nil
}

func (p *Ising) SendConsensusMsg(payload serialization.SerializableData) error {
	isingPld, err := BuildIsingPayload(payload)
	if err != nil {
		return err
	}
	err = p.localNode.Xmit(isingPld)
	if err != nil {
		return err
	}

	return nil
}

func CreateBookkeepingTransaction() *transaction.Transaction {
	bookKeepingPayload := &payload.BookKeeping{
		Nonce: rand.Uint64(),
	}
	return &transaction.Transaction{
		TxType:         transaction.BookKeeping,
		PayloadVersion: payload.BookKeepingPayloadVersion,
		Payload:        bookKeepingPayload,
		Attributes:     []*transaction.TxAttribute{},
		UTXOInputs:     []*transaction.UTXOTxInput{},
		Outputs:        []*transaction.TxOutput{},
		Programs:       []*program.Program{},
	}
}

func (p *Ising) BuildBlock() (*ledger.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []Uint256
	coinbase := CreateBookkeepingTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	txns := p.txnCollector.Collect()
	for txnHash, txn := range txns {
		txnList = append(txnList, txn)
		txnHashList = append(txnHashList, txnHash)
	}
	txnRoot, err := crypto.ComputeRoot(txnHashList)
	if err != nil {
		return nil, err
	}
	header := &ledger.Header{
		TransactionsRoot: txnRoot,
		Program: &program.Program{
			Code:      []byte{},
			Parameter: []byte{},
		},
	}
	block := &ledger.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}

func (p *Ising) ReceiveConsensusMsg(v interface{}) {
	if payload, ok := v.(*message.IsingPayload); ok {
		isingMsg, err := RecoverFromIsingPayload(payload.PayloadData)
		if err != nil {
			fmt.Println("Deserialization of ising message error")
		}
		switch t := isingMsg.(type) {
		case *BlockFlooding:
			p.HandleBlockFloodingMsg(t)
		case *BlockProposal:
			p.HandleBlockProposalMsg(t)
		case *BlockRequest:
			p.HandleBlockRequestMsg(t)
		case *BlockResponse:
			p.HandleBlockResponseMsg(t)
		case *BlockVote:
			p.HandleBlockVoteMsg(t)
		}
	}
}

func (p *Ising) HandleBlockFloodingMsg(bfMsg *BlockFlooding) {
	if !p.state.HasBit(InitialState) || p.state.HasBit(FloodingFinished) {
		log.Warn("consensus state error in BlockFlooding message handler")
		return
	}
	err := p.blockCache.AddBlockToCache(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
	p.state.SetBit(FloodingFinished)
}

func (p *Ising) HandleBlockProposalMsg(bpMsg *BlockProposal) {
	if !p.state.HasBit(InitialState) {
		log.Warn("consensus state error in BlockProposal message handler")
		return
	}
	//TODO verify consensus message
	account, err := p.wallet.GetDefaultAccount()
	if err != nil {
		log.Error("local account error")
		return
	}
	hash := *bpMsg.blockHash
	if !p.state.HasBit(FloodingFinished) {
		if !p.blockCache.BlockInCache(hash) {
			brMsg := &BlockRequest{
				blockHash: bpMsg.blockHash,
				requester: account.PublicKey,
				signature: MsgSignatureStub,
			}
			p.SendConsensusMsg(brMsg)
			p.state.SetBit(RequestSent)
			return
		}
		p.state.SetBit(FloodingFinished)
		return
	} else {
		block := p.blockCache.GetBlockFromCache(hash)
		if block == nil {
			return
		}
		//TODO verify block
		option := true
		blMsg := &BlockVote{
			blockHash: bpMsg.blockHash,
			agree:     option,
			voter:     account.PublicKey,
			signature: MsgSignatureStub,
		}
		p.SendConsensusMsg(blMsg)
		p.state.SetBit(OpinionSent)
		p.blockCache.RemoveBlockFromCache(hash)
		return
	}
}

func (p *Ising) HandleBlockRequestMsg(brMsg *BlockRequest) {
	if !p.state.HasBit(InitialState) || !p.state.HasBit(FloodingFinished) || !p.state.HasBit(ProposalSent) {
		log.Warn("consensus state error in BlockRequest message handler")
		return
	}
	//TODO verify request message
	hash := *brMsg.blockHash
	if hash.CompareTo(*p.confirmingBlock) != 0 {
		log.Warn("requested block doesn't match with local block in process")
		return
	}
	b := p.blockCache.GetBlockFromCache(hash)
	if b == nil {
		return
	}
	respMsg := &BlockResponse{
		block: b,
	}
	p.SendConsensusMsg(respMsg)
	return
}

func (p *Ising) HandleBlockResponseMsg(brMsg *BlockResponse) {
	if !p.state.HasBit(InitialState) || !p.state.HasBit(RequestSent) {
		log.Warn("consensus state error in BlockResponse message handler")
		return
	}
	// TODO verify proposer
	err := p.blockCache.AddBlockToCache(brMsg.block)
	if err != nil {
		return
	}
	p.state.SetBit(FloodingFinished)
	account, err := p.wallet.GetDefaultAccount()
	if err != nil {
		log.Error("local account error")
		return
	}
	// TODO verify block
	option := true
	hash := brMsg.block.Hash()
	bvMsg := &BlockVote{
		blockHash: &hash,
		agree:     option,
		voter:     account.PublicKey,
		signature: MsgSignatureStub,
	}
	p.SendConsensusMsg(bvMsg)
	p.state.SetBit(OpinionSent)
	p.blockCache.RemoveBlockFromCache(hash)
	return
}

func (p *Ising) HandleBlockVoteMsg(bvMsg *BlockVote) {
	if !p.state.HasBit(InitialState) || !p.state.HasBit(FloodingFinished) || !p.state.HasBit(ProposalSent) {
		log.Warn("consensus state error in BlockVote message handler")
		return
	}
	//TODO verify blockvote message
	hash := bvMsg.blockHash
	if hash.CompareTo(*p.confirmingBlock) != 0 {
		log.Warn("voted block doesn't match with local block in process")
		return
	}
	if bvMsg.agree == true {
		p.votedNum++
	}
	return
}
