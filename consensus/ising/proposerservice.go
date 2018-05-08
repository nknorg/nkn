package ising

import (
	"fmt"
	"math/rand"
	"time"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
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
	ConsensusTime                   = 6 * time.Second
	WaitingForBlockFloodingDuration = ConsensusTime / FloodingFactor
	WaitingForBlockVoteDuration     = ConsensusTime / VoteFactor
)

type ProposerService struct {
	account              *wallet.Account           // local account
	ticker               *time.Ticker              // ticker for proposal service
	spreader             bool                      // whether the node could do block flooding
	localNode            protocol.Noder            // local node
	txnCollector         *transaction.TxnCollector // collect transaction from where
	totalWeight          int                       // neighbors total weight
	agreedWeight         int                       // agreed weight
	msgChan              chan interface{}          // get notice from probe thread
	consensusMsgReceived events.Subscriber         // consensus events listening
	index                int                       // index for voting
	voting               []voting.Voting           // array for sigchain and block voting
}

func NewProposerService(account *wallet.Account, node protocol.Noder) *ProposerService {
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
		ticker:       time.NewTicker(ConsensusTime),
		spreader:     flag,
		account:      account,
		localNode:    node,
		txnCollector: transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged),
		msgChan:      make(chan interface{}, MsgChanCap),
		index:        0,
		voting:       []voting.Voting{voting.NewBlockVoting()},
	}

	return service
}

func (ps *ProposerService) CurrentVoting() voting.Voting {
	return ps.voting[ps.index]
}

func (ps *ProposerService) ProposerRoutine() {
	var err error
	var block *ledger.Block
	var current = ps.CurrentVoting()
	if ps.spreader {
		block, err = ps.BuildBlock()
		if err != nil {
			log.Error("building block error: ", err)
		}
		err = ps.CurrentVoting().Preparing(block)
		if err != nil {
			log.Error("adding block to cache error: ", err)
		}
		blockFlooding := NewBlockFlooding(block)
		err = ps.SendConsensusMsg(blockFlooding, nil)
		if err != nil {
			log.Error("sending consensus message error: ", err)
		}
		current.SetProposerState(block.Hash(), voting.FloodingFinished)
		neighbors := ps.localNode.GetNeighborNoder()
		for _, v := range neighbors {
			id := publickKeyToNodeID(v.GetPubKey())
			current.SetVoterState(id, block.Hash(), voting.FloodingFinished)
		}
	}
	// waiting for other nodes spreader finished
	time.Sleep(WaitingForBlockFloodingDuration)
	content, err := current.GetCurrentVotingContent()
	if err != nil {
		return
	}
	hash := content.Hash()
	proposalMsg := NewProposal(&hash, current.VotingType())
	current.SetProposerState(hash, voting.ProposalSent)
	current.SetConfirmingHash(hash)
	ps.SendConsensusMsg(proposalMsg, nil)
	ps.totalWeight = GetNeighborsVotingWeight(ps.localNode.GetNeighborNoder())

	// waiting for other nodes voting finished
	time.Sleep(WaitingForBlockVoteDuration)
	if ps.agreedWeight <= ps.totalWeight/2 {
		ps.Reset()
		return
	}

	if current.VotingType() == voting.BlockVote {
		err = ledger.DefaultLedger.Blockchain.AddBlock(content.(*ledger.Block))
		if err != nil {
			log.Error("saving block error: ", err)
		}
	}
	ps.Reset()

	return
}

func (ps *ProposerService) Start() error {
	ps.consensusMsgReceived = ps.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, ps.ReceiveConsensusMsg)
	go func() {
		for {
			select {
			case <-ps.ticker.C:
				for k := range ps.voting {
					ps.index = k
					ps.ProposerRoutine()
					time.Sleep(time.Second * 2)
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case msg := <-ps.msgChan:
				if notice, ok := msg.(*Notice); ok {
					heightHashMap := make(map[uint32]Uint256)
					heightNeighborMap := make(map[uint32]uint64)
					for k, v := range notice.BlockHistory {
						height, hash := StringToHeightHash(k)
						heightHashMap[height] = hash
						heightNeighborMap[height] = v
					}
					if height, ok := ledger.DefaultLedger.Store.CheckBlockHistory(heightHashMap); !ok {
						//TODO DB reverts to 'height' - 1 and request blocks from neighbor n[height]
						_ = height
					}
				}
			}
		}
	}()

	return nil
}

func (ps *ProposerService) SendConsensusMsg(msg IsingMessage, to *crypto.PubKey) error {
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

	// send message to all neighbors
	if to == nil {
		err = ps.localNode.Xmit(isingPld)
		if err != nil {
			return err
		}
		return nil
	}
	neighbors := ps.localNode.GetNeighborNoder()
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

func (ps *ProposerService) BuildBlock() (*ledger.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []Uint256
	coinbase := CreateBookkeepingTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	txns := ps.txnCollector.Collect()
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

func (ps *ProposerService) ReceiveConsensusMsg(v interface{}) {
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
			ps.HandleBlockFloodingMsg(t, sender)
		case *Request:
			ps.HandleRequestMsg(t, sender)
		case *Response:
			ps.HandleResponseMsg(t, sender)
		case *Voting:
			ps.HandleVoteMsg(t, sender)
		case *StateProbe:
			ps.HandleStateProbeMsg(t, sender)
		case *Proposal:
			ps.HandleProposalMsg(t, sender)
		}
	}
}

func (ps *ProposerService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender *crypto.PubKey) {
	blockHash := bfMsg.block.Hash()
	current := ps.CurrentVoting()
	if current.HasProposerState(blockHash, voting.FloodingFinished) {
		log.Warn("consensus state error in BlockFlooding message handler")
		return
	}
	// TODO check if the sender is PoR node
	err := current.Preparing(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
	current.SetProposerState(blockHash, voting.FloodingFinished)
	neighbors := ps.localNode.GetNeighborNoder()
	for _, v := range neighbors {
		id := publickKeyToNodeID(v.GetPubKey())
		current.SetVoterState(id, bfMsg.block.Hash(), voting.FloodingFinished)
	}
	current.DumpState(blockHash, "after handle block flooding", false)
	return
}

func (ps *ProposerService) HandleRequestMsg(brMsg *Request, sender *crypto.PubKey) {
	hash := *brMsg.hash
	current := ps.CurrentVoting()
	current.DumpState(hash, "when handle request", false)

	if !current.HasProposerState(hash, voting.FloodingFinished) {
		log.Warn("consensus state error in Request message handler")
		return
	}
	// TODO check if already sent Proposal to sender
	if hash.CompareTo(current.GetConfirmingHash()) != 0 {
		log.Warn("requested block doesn't match with local block in process")
		return
	}
	content, err := current.GetVotingContent(hash)
	if err != nil {
		return
	}
	responseMsg := NewResponse(&hash, current.VotingType(), content)
	ps.SendConsensusMsg(responseMsg, sender)
	return
}

func (ps *ProposerService) HandleVoteMsg(bvMsg *Voting, sender *crypto.PubKey) {
	blockHash := *bvMsg.hash
	current := ps.CurrentVoting()
	current.DumpState(blockHash, "when handle block voting", false)
	if !current.HasProposerState(blockHash, voting.FloodingFinished) || !current.HasProposerState(blockHash, voting.ProposalSent) {
		log.Warn("consensus state error in Voting message handler")
		return
	}
	// TODO check if the sender is neighbor
	hash := bvMsg.hash
	if hash.CompareTo(current.GetConfirmingHash()) != 0 {
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
		ps.agreedWeight += weight
	}
	return
}

func (ps *ProposerService) Reset() {
	ps.totalWeight = GetNeighborsVotingWeight(ps.localNode.GetNeighborNoder())
	ps.agreedWeight = 0
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

func (ps *ProposerService) HandleStateProbeMsg(msg *StateProbe, sender *crypto.PubKey) {
	switch msg.ProbeType {
	case BlockHistory:
		switch t := msg.ProbePayload.(type) {
		case *BlockHistoryPayload:
			history := ledger.DefaultLedger.Store.GetBlockHistory(t.StartHeight, t.StartHeight+t.BlockNum)
			s := &StateResponse{history}
			ps.SendConsensusMsg(s, sender)
		}
	}
	return
}

func (ps *ProposerService) HandleResponseMsg(brMsg *Response, sender *crypto.PubKey) {
	hash := brMsg.hash
	nodeID := publickKeyToNodeID(sender)
	current := ps.CurrentVoting()
	if !current.HasVoterState(nodeID, *hash, voting.RequestSent) {
		log.Warn("consensus state error in Response message handler")
		return
	}
	// TODO check if the sender is requested neighbor node
	err := current.Preparing(brMsg.content)
	if err != nil {
		return
	}
	current.SetVoterState(nodeID, *hash, voting.FloodingFinished)
	// TODO verify block
	agree := true
	votingMsg := NewVoting(hash, agree)
	ps.SendConsensusMsg(votingMsg, sender)
	current.SetVoterState(nodeID, *hash, voting.OpinionSent)
}

func (ps *ProposerService) HandleProposalMsg(bpMsg *Proposal, sender *crypto.PubKey) {
	// TODO check if the sender is neighbor
	nodeID := publickKeyToNodeID(sender)
	hash := *bpMsg.hash
	current := ps.CurrentVoting()
	if current.HasVoterState(nodeID, hash, voting.OpinionSent) {
		log.Warn("consensus state error in Proposal message handler")
		return
	}
	if !current.Exist(hash) {
		requestMsg := NewRequest(&hash, current.VotingType())
		ps.SendConsensusMsg(requestMsg, sender)
		current.SetVoterState(nodeID, hash, voting.RequestSent)
		log.Warn("doesn't contain block in local cache, requesting it from neighbor")
		return
	}

	if !current.HasVoterState(nodeID, hash, voting.FloodingFinished) {
		log.Warn("require FloodingFinished state in Proposal message handler")
		return
	}
	//TODO block verification and mind changing
	agree := true
	votingMsg := NewVoting(&hash, agree)
	ps.SendConsensusMsg(votingMsg, sender)
	current.SetVoterState(nodeID, hash, voting.OpinionSent)
}
