package ising

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	TxnAmountToBePackaged           = 1024
	FloodingFactor                  = 5
	VoteFactor                      = 5
	ConsensusTime                   = 10 * time.Second
	WaitingForBlockFloodingDuration = ConsensusTime / FloodingFactor
	WaitingForVotingDuration        = ConsensusTime / VoteFactor
	WaitingForNextRound             = time.Second
)

type ProposerService struct {
	account              *wallet.Account           // local account
	ticker               *time.Ticker              // ticker for proposal service
	blockProducer        []byte                    // whether the node could do block flooding
	localNode            protocol.Noder            // local node
	txnCollector         *transaction.TxnCollector // collect transaction from where
	height               uint32                    // current height in consensus
	msgChan              chan interface{}          // get notice from probe thread
	consensusMsgReceived events.Subscriber         // consensus events listening
	porServer            *por.PorServer
	voting               []voting.Voting          // array for sigchain and block voting
	votingChan           map[uint64]chan struct{} // used for waiting vote
}

func NewProposerService(account *wallet.Account, node protocol.Noder, porServer *por.PorServer) *ProposerService {
	minEncodedPubKey, _ := ledger.StandbyBookKeepers[0].EncodePoint(true)
	totalWeight := GetTotalVotingWeight(node)
	txnCollector := transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged)

	votingChan := make(map[uint64]chan struct{})
	neighbors := node.GetNeighborNoder()
	for _, v := range neighbors {
		votingChan[v.GetID()] = make(chan struct{})
	}
	service := &ProposerService{
		ticker:        time.NewTicker(ConsensusTime),
		blockProducer: minEncodedPubKey,
		account:       account,
		localNode:     node,
		txnCollector:  txnCollector,
		height:        ledger.DefaultLedger.Store.GetHeight() + 1,
		msgChan:       make(chan interface{}, MsgChanCap),
		voting: []voting.Voting{
			voting.NewSigChainVoting(totalWeight, porServer, txnCollector),
			voting.NewBlockVoting(totalWeight),
		},
		votingChan: votingChan,
	}

	return service
}

func (ps *ProposerService) CurrentVoting(vType voting.VotingContentType) voting.Voting {
	for _, v := range ps.voting {
		if v.VotingType() == vType {
			return v
		}
	}

	return nil
}

func (ps *ProposerService) ProposerRoutine(vType voting.VotingContentType) {
	time.Sleep(WaitingForBlockFloodingDuration)
	// send new proposal
	content, err := ps.SendNewProposal(vType)
	if err != nil {
		log.Info("waiting for receiving proposed entity...")
		return
	}
	// waiting for voting finished
	time.Sleep(WaitingForVotingDuration)
	hash, err := ps.VoteCounting(vType)
	if err != nil {
		log.Warn("vote counting error: ", err)
		return
	}
	current := ps.CurrentVoting(vType)
	// if proposed hash is not original entity, then get it from local cache
	if hash.CompareTo(content.Hash()) != 0 {
		content, err = current.GetVotingContent(*hash)
		if err != nil {
			log.Warn("get final entity error")
			return
		}
	}
	// process final block and signature chain
	switch vType {
	case voting.SigChainVote:
		sigChain := content.(*por.SigChain)
		// TODO: get a determinate public key on signature chain
		pbk, _ := sigChain.GetLastPubkey()
		ps.blockProducer = pbk
		sigChainHash := content.Hash()
		log.Info("sigature chain reach a consensus: ", BytesToHexString(sigChainHash.ToArray()))
	case voting.BlockVote:
		err = ledger.DefaultLedger.Blockchain.AddBlock(content.(*ledger.Block))
		if err != nil {
			log.Error("saving block error: ", err)
		}
		//TODO: transaction pool cleanup
	}
	time.Sleep(WaitingForNextRound)
	// since neighbor may changed, reset total voting weight
	ps.Reset(GetTotalVotingWeight(ps.localNode))

}
func (ps *ProposerService) SendNewProposal(vType voting.VotingContentType) (voting.VotingContent, error) {
	current := ps.CurrentVoting(vType)
	content, err := current.GetBestVotingContent()
	if err != nil {
		return nil, err
	}
	hash := content.Hash()
	log.Infof("proposing hash: %s, type: %d", BytesToHexString(hash.ToArray()), vType)
	// create new proposal
	proposalMsg := NewProposal(&hash, ps.height, vType)
	// send proposal to neighbors
	ps.SendConsensusMsg(proposalMsg, nil)
	// state changed for current hash
	current.SetProposerState(hash, voting.ProposalSent)
	// set confirming hash
	current.SetConfirmingHash(hash)
	// add self mind to voting pool
	current.GetVotingPool().SetMind(ps.height, hash)
	for _, v := range ps.votingChan {
		v <- struct{}{}
	}

	return content, nil
}

func (ps *ProposerService) VoteCounting(vType voting.VotingContentType) (*Uint256, error) {
	currentVotingPool := ps.CurrentVoting(vType).GetVotingPool()
	// get voting results from voting pool
	maybeFinalHash, err := currentVotingPool.VoteCounting(ps.height)
	if err != nil {
		return nil, err
	}
	// if current mind is different with voting result then change mind
	if currentVotingPool.NeedChangeMind(ps.height, *maybeFinalHash) {
		log.Infof("Mind changed to %s when received votes from neighbors\n",
			BytesToHexString(maybeFinalHash.ToArray()))
		currentVotingPool.ChangeMind(ps.height, *maybeFinalHash)
	}
	// TODO: change mind again

	return maybeFinalHash, nil
}

func (ps *ProposerService) SetConsensusHeight() {
	ps.height = ledger.DefaultLedger.Store.GetHeight() + 1
}

func (ps *ProposerService) ProduceNewBlock() {
	current := ps.CurrentVoting(voting.BlockVote)
	block, err := ps.BuildBlock()
	if err != nil {
		log.Error("building block error: ", err)
	}
	err = current.Preparing(block)
	if err != nil {
		log.Error("adding block to cache error: ", err)
		return
	}
	blockFlooding := NewBlockFlooding(block)
	err = ps.SendConsensusMsg(blockFlooding, nil)
	if err != nil {
		log.Error("sending consensus message error: ", err)
	}
	current.GetVotingPool().SetMind(ps.height, block.Hash())
}

func (ps *ProposerService) AmIBlockProducer() bool {
	localPublicKey, err := ps.account.PublicKey.EncodePoint(true)
	if err != nil {
		return false
	}
	if !IsEqualBytes(localPublicKey, ps.blockProducer) {
		return false
	}

	return true
}

func (ps *ProposerService) Start() error {
	ps.consensusMsgReceived = ps.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, ps.ReceiveConsensusMsg)
	go func() {
		for {
			select {
			case <-ps.ticker.C:
				ps.SetConsensusHeight()
				for _, v := range ps.voting {
					if _, ok := v.(*voting.BlockVoting); ok {
						if ps.AmIBlockProducer() {
							log.Info("I am Block Producer")
							ps.ProduceNewBlock()
						}
					}
					ps.ProposerRoutine(v.VotingType())
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
		case *Vote:
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
	height := bfMsg.block.Header.Height
	if height < ps.height {
		log.Warnf("receive out of date block, consensus height: %d, received block height: %d,"+
			" hash: %s\n", ps.height, height, BytesToHexString(blockHash.ToArray()))
		return
	}
	current := ps.CurrentVoting(voting.BlockVote)
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
	current.GetVotingPool().SetMind(height, blockHash)
}

func (ps *ProposerService) HandleRequestMsg(req *Request, sender *crypto.PubKey) {
	hash := *req.hash
	height := req.height
	if height < ps.height {
		log.Warnf("receive invalid request, consensus height: %d, request height: %d,"+
			" hash: %s\n", ps.height, height, BytesToHexString(hash.ToArray()))
		return
	}
	current := ps.CurrentVoting(req.contentType)
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
	responseMsg := NewResponse(&hash, height, current.VotingType(), content)
	ps.SendConsensusMsg(responseMsg, sender)
}

func (ps *ProposerService) HandleVoteMsg(vote *Vote, sender *crypto.PubKey) {
	hash := *vote.hash
	height := vote.height
	if height != ps.height {
		log.Warnf("receive invalid vote, consensus height: %d, vote height: %d,"+
			" hash: %s\n", ps.height, height, BytesToHexString(hash.ToArray()))
		return
	}
	current := ps.CurrentVoting(vote.contentType)
	current.DumpState(hash, "when handle voting message", false)
	if !current.HasProposerState(hash, voting.ProposalSent) {
		log.Warn("consensus state error in Vote message handler")
		return
	}
	// TODO check if the sender is neighbor
	if hash.CompareTo(current.GetConfirmingHash()) != 0 {
		log.Warn("voted block doesn't match with local block in process")
		return
	}
	nid := publickKeyToNodeID(sender)
	if vote.agree == true {
		current.GetVotingPool().AddToReceivePool(nid, height, hash)
	} else {
		current.GetVotingPool().AddToReceivePool(nid, height, *vote.preferHash)
	}
}

func (ps *ProposerService) Reset(totalWeight int) {
	// reset total voting weight
	for _, v := range ps.voting {
		v.GetVotingPool().Reset(totalWeight)
	}
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

func (ps *ProposerService) HandleResponseMsg(resp *Response, sender *crypto.PubKey) {
	hash := resp.hash
	height := resp.height
	if height != ps.height {
		log.Warnf("receive invalid response, consensus height: %d, response height: %d,"+
			" hash: %s\n", ps.height, height, BytesToHexString(hash.ToArray()))
		return
	}
	nodeID := publickKeyToNodeID(sender)
	current := ps.CurrentVoting(resp.contentType)
	if !current.HasVoterState(nodeID, *hash, voting.RequestSent) {
		log.Warn("consensus state error in Response message handler")
		return
	}
	// TODO check if the sender is requested neighbor node
	err := current.Preparing(resp.content)
	if err != nil {
		return
	}
	current.SetVoterState(nodeID, *hash, voting.FloodingFinished)
	currentVotingPool := current.GetVotingPool()
	<-ps.votingChan[nodeID]
	currentMind := currentVotingPool.GetMind(height)
	var votingMsg *Vote
	if hash.CompareTo(currentMind) != 0 {
		votingMsg = NewVoting(hash, height, resp.contentType, false, &currentMind)
	} else {
		votingMsg = NewVoting(hash, height, resp.contentType, true, nil)
	}
	ps.SendConsensusMsg(votingMsg, sender)
	current.SetVoterState(nodeID, *hash, voting.OpinionSent)
}

func (ps *ProposerService) HandleProposalMsg(proposal *Proposal, sender *crypto.PubKey) {
	hash := *proposal.hash
	height := proposal.height
	if height < ps.height-1 {
		log.Warnf("receive invalid proposal, consensus height: %d, proposal height: %d,"+
			" hash: %s\n", ps.height, height, BytesToHexString(hash.ToArray()))
		return
	}
	// TODO check if the sender is neighbor
	nodeID := publickKeyToNodeID(sender)
	current := ps.CurrentVoting(proposal.contentType)
	if current.HasVoterState(nodeID, hash, voting.OpinionSent) {
		log.Warn("consensus state error in Proposal message handler")
		return
	}
	if !current.Exist(hash) {
		requestMsg := NewRequest(&hash, height, current.VotingType())
		ps.SendConsensusMsg(requestMsg, sender)
		current.SetVoterState(nodeID, hash, voting.RequestSent)
		log.Warnf("doesn't contain hash in local cache, requesting it from neighbor %s\n",
			BytesToHexString(hash.ToArray()))
		return
	}

	// TODO: remove FloodingFinished state is a workaround for both sigchain and block consensus
	currentVotingPool := current.GetVotingPool()
	<-ps.votingChan[nodeID]
	currentMind := currentVotingPool.GetMind(height)
	var votingMsg *Vote
	if hash.CompareTo(currentMind) != 0 {
		votingMsg = NewVoting(&hash, height, proposal.contentType, false, &currentMind)
	} else {
		votingMsg = NewVoting(&hash, height, proposal.contentType, true, nil)
	}
	ps.SendConsensusMsg(votingMsg, sender)
	current.SetVoterState(nodeID, hash, voting.OpinionSent)
}
