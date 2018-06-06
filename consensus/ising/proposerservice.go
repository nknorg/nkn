package ising

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
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
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
)

const (
	TxnAmountToBePackaged      = 1024
	ConsensusTime              = 10 * time.Second
	WaitingForFloodingFinished = time.Second * 4
	WaitingForVoting           = time.Second * 5
)

type ProposerService struct {
	sync.RWMutex
	account              *wallet.Account           // local account
	ticker               *time.Ticker              // ticker for proposal service
	blockProposer        map[uint32][]byte         // height and public key mapping for block proposer
	localNode            protocol.Noder            // local node
	neighbors            []protocol.Noder          // neighbor nodes
	txnCollector         *transaction.TxnCollector // collect transaction from where
	msgChan              chan interface{}          // get notice from probe thread
	consensusMsgReceived events.Subscriber         // consensus events listening
	porServer            *por.PorServer            // signature chain source
	voting               []voting.Voting           // array for sigchain and block voting
}

func NewProposerService(account *wallet.Account, node protocol.Noder) *ProposerService {
	totalWeight := GetTotalVotingWeight(node)
	txnCollector := transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged)

	service := &ProposerService{
		ticker:        time.NewTicker(ConsensusTime),
		account:       account,
		localNode:     node,
		neighbors:     node.GetNeighborNoder(),
		blockProposer: make(map[uint32][]byte),
		txnCollector:  txnCollector,
		msgChan:       make(chan interface{}, MsgChanCap),
		voting: []voting.Voting{
			voting.NewBlockVoting(totalWeight),
			voting.NewSigChainVoting(totalWeight, txnCollector),
		},
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
	// initialization for height and voting weight
	ps.Initialize(vType, GetTotalVotingWeight(ps.localNode))
	if vType == voting.BlockVote {
		if ps.IsBlockProposer() {
			log.Info("-> I am Block Proposer")
			ps.ProduceNewBlock()
		}
	}
	// waiting for flooding finished
	time.Sleep(WaitingForFloodingFinished)
	// send new proposal
	content, err := ps.SendNewProposal(vType)
	if err != nil {
		log.Info("waiting for receiving proposed entity...")
		return
	}
	// waiting for voting finished
	time.Sleep(WaitingForVoting)
	current := ps.CurrentVoting(vType)
	votingPool := current.GetVotingPool()
	votingHeight := current.GetVotingHeight()
	finalHash, _ := votingPool.GetMind(votingHeight)
	// if proposed hash is not original entity, then get it from local cache
	if finalHash.CompareTo(content.Hash()) != 0 {
		content, err = current.GetVotingContentFromPool(finalHash, votingHeight)
		if err != nil {
			log.Warn("get final entity error")
			return
		}
	}
	// process final block and signature chain
	switch vType {
	case voting.SigChainTxnVote:
		txn := content.(*transaction.Transaction)
		payload := txn.Payload.(*payload.Commit)
		sigchain := &por.SigChain{}
		proto.Unmarshal(payload.SigChain, sigchain)
		// TODO: get a determinate public key on signature chain
		pbk, err := sigchain.GetLastPubkey()
		if err != nil {
			log.Warn("Get last public key error", err)
			return
		}
		ps.Lock()
		ps.blockProposer[votingHeight] = pbk
		ps.Unlock()
		sigChainTxnHash := content.Hash()
		log.Info("sigchain transaction consensus: ", BytesToHexString(sigChainTxnHash.ToArray()))
	case voting.BlockVote:
		if block, ok := content.(*ledger.Block); ok {
			err = ledger.DefaultLedger.Blockchain.AddBlock(block)
			if err != nil {
				log.Error("saving block error: ", err)
				return
			}
			ps.txnCollector.Cleanup(block.Transactions)
		}
	}
}

// GetReceiverNode returns neighbors nodes according to neighbor node ID passed in.
// If 'nids' passed in is nil then returns all neighbor nodes.
func (ps *ProposerService) GetReceiverNode(nids []uint64) []protocol.Noder {
	if nids == nil {
		return ps.neighbors
	}
	var nodes []protocol.Noder
	for _, id := range nids {
		for _, node := range ps.neighbors {
			if id == node.GetID() {
				nodes = append(nodes, node)
			}
		}
	}

	return nodes
}

func (ps *ProposerService) SendNewProposal(vType voting.VotingContentType) (voting.VotingContent, error) {
	current := ps.CurrentVoting(vType)
	votingHeight := current.GetVotingHeight()
	content, err := current.GetBestVotingContent(votingHeight)
	if err != nil {
		return nil, err
	}
	hash := content.Hash()
	votingPool := current.GetVotingPool()
	if mind, ok := votingPool.GetMind(votingHeight); ok {
		// if local hash doesn't better than
		if mind.CompareTo(hash) == -1 {
			hash = mind
		}
	} else {
		// set mind if it has not been set
		votingPool.ChangeMind(votingHeight, hash)
	}
	if !current.HasSelfState(hash, voting.ProposalSent) {
		log.Infof("proposing hash: %s, type: %d", BytesToHexString(hash.ToArray()), vType)
		// create new proposal
		proposalMsg := NewProposal(&hash, votingHeight, vType)
		// get nodes which should receive proposal message
		nodes := ps.GetReceiverNode(nil)
		// send proposal to neighbors
		ps.SendConsensusMsg(proposalMsg, nodes)
		// state changed for current hash
		current.SetSelfState(hash, voting.ProposalSent)
		// set confirming hash
		current.SetConfirmingHash(hash)
	}

	return content, nil
}

func (ps *ProposerService) ProduceNewBlock() {
	current := ps.CurrentVoting(voting.BlockVote)
	votingPool := current.GetVotingPool()
	votingHeight := current.GetVotingHeight()
	// build new block to be proposed
	block, err := ps.BuildBlock()
	if err != nil {
		log.Error("building block error: ", err)
	}
	err = current.Preparing(block)
	if err != nil {
		log.Error("adding block to cache error: ", err)
		return
	}
	// generate BlockFlooding message
	blockFlooding := NewBlockFlooding(block)
	// get nodes which should receive this message
	nodes := ps.GetReceiverNode(nil)
	// send BlockFlooding message
	err = ps.SendConsensusMsg(blockFlooding, nodes)
	if err != nil {
		log.Error("sending consensus message error: ", err)
	}
	// update mind of local node
	votingPool.ChangeMind(votingHeight, block.Hash())
}

func (ps *ProposerService) IsBlockProposer() bool {
	localPublicKey, err := ps.account.PublicKey.EncodePoint(true)
	if err != nil {
		return false
	}
	current := ps.CurrentVoting(voting.BlockVote)
	votingHeight := current.GetVotingHeight()
	var proposer []byte
	if v, ok := ps.blockProposer[votingHeight]; ok {
		proposer = v
		log.Infof("Block Proposer (signature chain): %s", BytesToHexString(v))
	} else {
		proposer, _ = HexStringToBytes(config.Parameters.BlockProposer[0])
	}
	if IsEqualBytes(localPublicKey, proposer) {
		return true
	}

	return false
}

func (ps *ProposerService) Start() error {
	ps.consensusMsgReceived = ps.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, ps.ReceiveConsensusMsg)
	go func() {
		for {
			select {
			case <-ps.ticker.C:
				for _, v := range ps.voting {
					go ps.ProposerRoutine(v.VotingType())
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

func (ps *ProposerService) SendConsensusMsg(msg IsingMessage, to []protocol.Noder) error {
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

	for _, node := range to {
		b, err := message.NewIsingConsensus(isingPld)
		if err != nil {
			return err
		}
		node.Tx(b)
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
		case *StateProbe:
			ps.HandleStateProbeMsg(t, sender)
		case *Proposal:
			ps.HandleProposalMsg(t, sender)
		case *MindChanging:
			ps.HandleMindChangingMsg(t, sender)
		}
	}
}

func (ps *ProposerService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender *crypto.PubKey) {
	current := ps.CurrentVoting(voting.BlockVote)
	votingHeight := current.GetVotingHeight()
	blockHash := bfMsg.block.Hash()
	height := bfMsg.block.Header.Height
	if height < votingHeight {
		log.Warnf("receive out of date block, consensus height: %d, received block height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(blockHash.ToArray()))
		return
	}
	// returns if receive duplicate block
	if current.HasSelfState(blockHash, voting.FloodingFinished) {
		log.Warn("Duplicate block received for hash: ", BytesToHexString(blockHash.ToArray()))
		return
	}
	// set state for flooding block
	current.SetSelfState(blockHash, voting.FloodingFinished)

	// TODO check if the sender is PoR node
	err := current.Preparing(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
}

func (ps *ProposerService) HandleRequestMsg(req *Request, sender *crypto.PubKey) {
	current := ps.CurrentVoting(req.contentType)
	votingHeight := current.GetVotingHeight()
	hash := *req.hash
	height := req.height
	if height < votingHeight {
		log.Warnf("receive invalid request, consensus height: %d, request height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArray()))
		return
	}
	// TODO check if already sent Proposal to sender
	if hash.CompareTo(current.GetConfirmingHash()) != 0 {
		log.Warn("requested block doesn't match with local block in process")
		return
	}
	if !current.HasSelfState(hash, voting.ProposalSent) {
		log.Warn("receive invalid request for hash: ", BytesToHexString(hash.ToArray()))
		return
	}
	nodeID := publickKeyToNodeID(sender)
	// returns if receive duplicate request
	if current.HasNeighborState(nodeID, hash, voting.RequestReceived) {
		log.Warn("duplicate request received for hash: ", BytesToHexString(hash.ToArray()))
		return
	}
	// set state for request
	current.SetNeighborState(nodeID, hash, voting.RequestReceived)
	content, err := current.GetVotingContent(hash, height)
	if err != nil {
		return
	}
	// generate response message
	responseMsg := NewResponse(&hash, height, current.VotingType(), content)
	// get node which should receive response message
	nodes := ps.GetReceiverNode([]uint64{publickKeyToNodeID(sender)})
	// send response message
	ps.SendConsensusMsg(responseMsg, nodes)
}

func (ps *ProposerService) Initialize(vType voting.VotingContentType, totalWeight int) {
	// initial total voting weight
	for _, v := range ps.voting {
		v.GetVotingPool().Reset(totalWeight)
	}
	// initial voting height
	current := ps.CurrentVoting(vType)
	current.UpdateVotingHeight()
	// initial neighbor nodes
	ps.neighbors = ps.localNode.GetNeighborNoder()
}

func (ps *ProposerService) HandleStateProbeMsg(msg *StateProbe, sender *crypto.PubKey) {
	switch msg.ProbeType {
	case BlockHistory:
		switch t := msg.ProbePayload.(type) {
		case *BlockHistoryPayload:
			history := ledger.DefaultLedger.Store.GetBlockHistory(t.StartHeight, t.StartHeight+t.BlockNum)
			s := &StateResponse{history}
			nodes := ps.GetReceiverNode([]uint64{publickKeyToNodeID(sender)})
			ps.SendConsensusMsg(s, nodes)
		}
	}
	return
}

func (ps *ProposerService) HandleResponseMsg(resp *Response, sender *crypto.PubKey) {
	current := ps.CurrentVoting(resp.contentType)
	votingHeight := current.GetVotingHeight()
	hash := resp.hash
	height := resp.height
	if height != votingHeight {
		log.Warnf("receive invalid response, consensus height: %d, response height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArray()))
		return
	}
	nodeID := publickKeyToNodeID(sender)
	// returns if no request sent before
	if !current.HasNeighborState(nodeID, *hash, voting.RequestSent) {
		log.Warn("consensus state error in Response message handler")
		return
	}
	// returns if receive duplicate response
	if current.HasNeighborState(nodeID, *hash, voting.ProposalReceived) {
		log.Warn("duplicate response received for hash: ", BytesToHexString(hash.ToArray()))
		return
	}
	// TODO check if the sender is requested neighbor node
	err := current.Preparing(resp.content)
	if err != nil {
		return
	}
	currentVotingPool := current.GetVotingPool()
	currentVotingPool.AddToReceivePool(height, nodeID, *hash)
	current.SetNeighborState(nodeID, *hash, voting.ProposalReceived)
}

func (ps *ProposerService) HandleProposalMsg(proposal *Proposal, sender *crypto.PubKey) {
	current := ps.CurrentVoting(proposal.contentType)
	votingType := current.VotingType()
	votingHeight := current.GetVotingHeight()
	hash := *proposal.hash
	height := proposal.height
	if height < votingHeight-1 {
		log.Warnf("receive invalid proposal, consensus height: %d, proposal height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArray()))
		return
	}
	// TODO check if the sender is neighbor
	nodeID := publickKeyToNodeID(sender)
	if !current.Exist(hash, height) {
		// generate request message
		requestMsg := NewRequest(&hash, height, votingType)
		// get node which should receive request message
		nodes := ps.GetReceiverNode([]uint64{nodeID})
		// send request message
		ps.SendConsensusMsg(requestMsg, nodes)
		current.SetNeighborState(nodeID, hash, voting.RequestSent)
		log.Warnf("doesn't contain hash in local cache, requesting it from neighbor %s\n",
			BytesToHexString(hash.ToArray()))
		return
	}
	// returns if receive duplicated proposal
	if current.HasNeighborState(nodeID, hash, voting.ProposalReceived) {
		log.Warn("duplicate proposal received for hash: ", BytesToHexString(hash.ToArray()))
		return
	}
	// set state when receive a proposal from a neighbor
	current.SetNeighborState(nodeID, hash, voting.ProposalReceived)
	currentVotingPool := current.GetVotingPool()
	currentVotingPool.AddToReceivePool(height, nodeID, hash)

	// Get voting result from voting pool. If votes is not enough then return.
	maybeFinalHash, err := currentVotingPool.VoteCounting(votingHeight)
	if err != nil {
		return
	}
	if mind, ok := currentVotingPool.GetMind(votingHeight); ok {
		// When current mind has been set, if voting result is different with
		// current mind then do mind changing.
		if mind.CompareTo(*maybeFinalHash) != 0 {
			history := currentVotingPool.ChangeMind(votingHeight, *maybeFinalHash)
			// generate mind changing message
			mindChangingMsg := NewMindChanging(maybeFinalHash, votingHeight, votingType)
			// get node which should receive request message
			var nids []uint64
			for n := range history {
				nids = append(nids, n)
			}
			nodes := ps.GetReceiverNode(nids)
			// send mind changing message
			ps.SendConsensusMsg(mindChangingMsg, nodes)
		}
	} else {
		// Set mind if current mind has not been set.
		currentVotingPool.ChangeMind(votingHeight, *maybeFinalHash)
	}
}

func (ps *ProposerService) HandleMindChangingMsg(mindChanging *MindChanging, sender *crypto.PubKey) {
	current := ps.CurrentVoting(mindChanging.contentType)
	votingType := current.VotingType()
	votingHeight := current.GetVotingHeight()
	hash := *mindChanging.hash
	height := mindChanging.height
	if height < votingHeight {
		log.Warnf("receive invalid mind changing, consensus height: %d, mind changing height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArray()))
		return
	}
	// TODO check if the sender is neighbor
	nodeID := publickKeyToNodeID(sender)
	currentVotingPool := current.GetVotingPool()
	receivePool := currentVotingPool.GetReceivePool(votingHeight)
	if _, ok := receivePool[nodeID]; !ok {
		log.Warn("no proposal received before, so mind changing is invalid")
		return
	}
	// update voting pool when receive valid mind changing message
	currentVotingPool.AddToReceivePool(height, nodeID, hash)

	// recalculate votes
	maybeFinalHash, err := currentVotingPool.VoteCounting(votingHeight)
	if err != nil {
		return
	}
	if mind, ok := currentVotingPool.GetMind(votingHeight); ok {
		// When current mind has been set, if voting result is different with
		// current mind then do mind changing.
		if mind.CompareTo(*maybeFinalHash) != 0 {
			history := currentVotingPool.ChangeMind(votingHeight, *maybeFinalHash)
			// generate mind changing message
			mindChangingMsg := NewMindChanging(maybeFinalHash, votingHeight, votingType)
			// get node which should receive request message
			var nids []uint64
			for n := range history {
				nids = append(nids, n)
			}
			nodes := ps.GetReceiverNode(nids)
			// send mind changing message
			ps.SendConsensusMsg(mindChangingMsg, nodes)
		}
	}
}
