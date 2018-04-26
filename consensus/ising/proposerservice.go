package ising

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/contract"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	TxnAmountToBePackaged           = 1024
	FloodingFactor                  = 3
	VoteFactor                      = 3
	ConsensusTime                   = 3 * time.Second
	WaitingForBlockFloodingDuration = ConsensusTime / FloodingFactor
	WaitingForBlockVoteDuration     = ConsensusTime / VoteFactor
)

type ProposerService struct {
	account              *wallet.Account           // local account
	ticker               *time.Ticker              // ticker for proposal service
	spreader             bool                      // whether the node could do block flooding
	state                map[Uint256]*State        // consensus state
	localNode            protocol.Noder            // local node
	txnCollector         *transaction.TxnCollector // collect transaction from where
	confirmingBlock      *Uint256                  // current block in consensus process
	totalWeight          int                       // neighbors total weight
	agreedWeight         int                       // agreed weight
	blockCache           *BlockCache               // blocks waiting for voting
	consensusMsgReceived events.Subscriber         // consensus events listening
	msgChan              chan interface{}          // get notice from probe thread
	blockChan            chan *ledger.Block        // send block to voter thread cache
}

func NewProposerService(account *wallet.Account, node protocol.Noder, ch chan interface{}, bch chan *ledger.Block) *ProposerService {
	flag := false
	encPubKey, err := account.PublicKey.EncodePoint(true)
	if err != nil {
		return nil
	}
	confPubKey, err := ledger.StandbyBookKeepers[0].EncodePoint(true)
	if IsEqualBytes(encPubKey, confPubKey) {
		flag = true
	}

	service := &ProposerService{
		state:        make(map[Uint256]*State),
		ticker:       time.NewTicker(ConsensusTime),
		spreader:     flag,
		account:      account,
		localNode:    node,
		txnCollector: transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged),
		blockCache:   NewCache(),
		msgChan:      ch,
		blockChan:    bch,
	}

	return service
}

func (p *ProposerService) ProposerRoutine() {
	var err error
	var block *ledger.Block
	if p.spreader {
		block, err = p.BuildBlock()
		if err != nil {
			log.Error("building block error: ", err)
		}
		err = p.blockCache.AddBlockToCache(block)
		if err != nil {
			log.Error("adding block to cache error: ", err)
		}
		blockFlooding := &BlockFlooding{
			block: block,
		}
		err = p.SendConsensusMsg(blockFlooding, nil)
		if err != nil {
			log.Error("sending consensus message error: ", err)
		}
		p.blockChan <- block
		p.SetBlockState(block.Hash(), FloodingFinished)
	}
	// waiting for other nodes spreader finished
	time.Sleep(WaitingForBlockFloodingDuration)
	block = p.blockCache.GetCurrentBlockFromCache()
	hash := block.Hash()
	bpMsg := &BlockProposal{
		blockHash: &hash,
	}

	p.SendConsensusMsg(bpMsg, nil)
	p.SetBlockState(hash, ProposalSent)
	p.confirmingBlock = &hash
	p.totalWeight = GetNeighborsVotingWeight(p.localNode.GetNeighborNoder())

	// waiting for other nodes voting finished
	time.Sleep(WaitingForBlockVoteDuration)
	if p.agreedWeight <= p.totalWeight/2 {
		p.blockCache.RemoveBlockFromCache(hash)
		p.Reset()
		return
	}

	err = ledger.DefaultLedger.Blockchain.AddBlock(block)
	if err != nil {
		log.Error("saving block error: ", err)
	}
	p.Reset()

	return
}

func (p *ProposerService) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ReceiveConsensusMsg)
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.ProposerRoutine()
			}
		}
	}()
	go func() {
		for {
			select {
			case msg := <-p.msgChan:
				if notice, ok := msg.(*BlockInfoNotice); ok {
					h := notice.hash
					// neighbor node starting
					if h.CompareTo(Uint256{}) == 0 {
						time.Sleep(time.Second * 3)
					} else {
						// new joined node
						if p.confirmingBlock == nil {
							brmsg := &BlockRequest{
								blockHash: &h,
							}
							// request confirming block
							p.SendConsensusMsg(brmsg, p.localNode.GetNeighborNoder()[0].GetPubKey())
							p.confirmingBlock = &h
						} else {
							if p.confirmingBlock.CompareTo(h) != 0 {
								//p.confirmingBlock = &h
							}
						}
					}
				}
			}
		}
	}()

	return nil
}

func (p *ProposerService) SendConsensusMsg(msg IsingMessage, to *crypto.PubKey) error {
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
	if to == nil {
		err = p.localNode.Xmit(isingPld)
		if err != nil {
			return err
		}
		return nil
	}
	neighbors := p.localNode.GetNeighborNoder()
	for _, n := range neighbors {
		if n.GetID() == publickKeyToNodeID(to) {
			b, err := message.NewIsingConsensus(isingPld)
			if err != nil {
				return err
			}
			n.Tx(b)
		}
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

func (p *ProposerService) BuildBlock() (*ledger.Block, error) {
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
		Version:          0,
		PrevBlockHash:    ledger.DefaultLedger.Store.GetCurrentBlockHash(),
		Timestamp:        uint32(time.Now().Unix()),
		Height:           ledger.DefaultLedger.Store.GetHeight() + 1,
		ConsensusData:    rand.Uint64(),
		TransactionsRoot: txnRoot,
		NextBookKeeper:   Uint160{},
		Program: &program.Program{
			Code:      []byte{0x00},
			Parameter: []byte{0x00},
		},
	}
	block := &ledger.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}

func (p *ProposerService) HasBlockState(blockhash Uint256, state State) bool {
	if v, ok := p.state[blockhash]; !ok || v == nil {
		return false
	} else {
		if v.HasBit(state) {
			return true
		}
		return false
	}
}

func (p *ProposerService) SetBlockState(blockhash Uint256, s State) {
	if _, ok := p.state[blockhash]; !ok {
		p.state[blockhash] = new(State)
	}
	p.state[blockhash].SetBit(s)
}

func (p *ProposerService) ReceiveConsensusMsg(v interface{}) {
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
		case *BlockRequest:
			p.HandleBlockRequestMsg(t, sender)
		case *BlockVote:
			p.HandleBlockVoteMsg(t, sender)
		case *StateProbe:
			p.HandleStateProbeMsg(t, sender)
		}
	}
}

func (p *ProposerService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender *crypto.PubKey) {
	blockHash := bfMsg.block.Hash()
	dumpState(sender, "ProposerService received BlockFlooding", p.state[blockHash])
	if p.HasBlockState(blockHash, FloodingFinished) {
		log.Warn("consensus state error in BlockFlooding message handler")
		return
	}
	// TODO check if the sender is PoR node
	err := p.blockCache.AddBlockToCache(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
	p.SetBlockState(blockHash, FloodingFinished)
	return
}

func (p *ProposerService) HandleBlockRequestMsg(brMsg *BlockRequest, sender *crypto.PubKey) {
	blockHash := *brMsg.blockHash
	dumpState(sender, "ProposerService received BlockRequest", p.state[blockHash])
	if !p.HasBlockState(blockHash, FloodingFinished) {
		log.Warn("consensus state error in BlockRequest message handler")
		return
	}
	// TODO check if already sent BlockProposal to sender
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
	p.SendConsensusMsg(respMsg, sender)
	return
}

func (p *ProposerService) HandleBlockVoteMsg(bvMsg *BlockVote, sender *crypto.PubKey) {
	blockHash := *bvMsg.blockHash
	dumpState(sender, "ProposerService received BlockVote", p.state[blockHash])
	if !p.HasBlockState(blockHash, FloodingFinished) || !p.HasBlockState(blockHash, ProposalSent) {
		log.Warn("consensus state error in BlockVote message handler")
		return
	}
	// TODO check if the sender is neighbor
	hash := bvMsg.blockHash
	if hash.CompareTo(*p.confirmingBlock) != 0 {
		log.Warn("voted block doesn't match with local block in process")
		return
	}
	if bvMsg.agree == true {
		script, err := contract.CreateSignatureRedeemScript(sender)
		if err != nil {
			log.Warn("sender public key to script error")
			return
		}
		programHash, err := ToCodeHash(script)
		if err != nil {
			log.Warn("sender script to hash error")
			return
		}
		weight, err := GetNeighborWeightFromDB(programHash)
		if err != nil {
			log.Warn("get sender weight error")
			return
		}
		p.agreedWeight += weight
	}
	return
}

func (p *ProposerService) Reset() {
	p.totalWeight = GetNeighborsVotingWeight(p.localNode.GetNeighborNoder())
	p.agreedWeight = 0
}

func GetNeighborWeightFromDB(programHash Uint160) (int, error) {
	// TODO get node voting weight from database
	return 1, nil
}

func GetNeighborsVotingWeight(neighbors []protocol.Noder) int {
	total := 0
	for _, n := range neighbors {
		script, err := contract.CreateSignatureRedeemScript(n.GetPubKey())
		if err != nil {
			continue
		}
		programHash, err := ToCodeHash(script)
		if err != nil {
			continue
		}
		weight, err := GetNeighborWeightFromDB(programHash)
		if err != nil {
			continue
		}
		total += weight
	}

	return total
}

func (p *ProposerService) HandleStateProbeMsg(msg *StateProbe, sender *crypto.PubKey) {
	var resp StateResponse
	if p.confirmingBlock != nil {
		resp.currentBlockHeight = 0 //Fixme to real height
		resp.currentBlockHash = *p.confirmingBlock
	}

	p.SendConsensusMsg(&resp, sender)
	return
}
