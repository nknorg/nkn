package ising

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/core/contract/program"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/core/transaction/payload"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"

	"github.com/golang/protobuf/proto"
)

const (
	TxnAmountToBePackaged      = 1024
	ConsensusTime              = 10 * time.Second
	WaitingForFloodingFinished = time.Second * 1
	WaitingForVotingFinished   = time.Second * 5
	TimeoutTolerance           = time.Second * 2
)

type ProposerService struct {
	sync.RWMutex
	account              *vault.Account            // local account
	timer                *time.Timer               // timer for proposer node
	timeout              *time.Timer               // timeout for next round consensus
	blockProposer        map[uint32][]byte         // height and public key mapping for block proposer
	localNode            protocol.Noder            // local node
	neighbors            []protocol.Noder          // neighbor nodes
	txnCollector         *transaction.TxnCollector // collect transaction from where
	msgChan              chan interface{}          // get notice from probe thread
	consensusMsgReceived events.Subscriber         // consensus events listening
	blockPersisted       events.Subscriber         // block saved events
	syncFinished         events.Subscriber         // block syncing finished event
	syncCache            *SyncCache                // cache for block syncing
	voting               []voting.Voting           // array for sigchain and block voting
}

func NewProposerService(account *vault.Account, node protocol.Noder) *ProposerService {
	totalWeight := GetTotalVotingWeight(node)
	txnCollector := transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged)

	service := &ProposerService{
		timer:         time.NewTimer(ConsensusTime),
		timeout:       time.NewTimer(ConsensusTime + TimeoutTolerance),
		account:       account,
		localNode:     node,
		neighbors:     node.GetNeighborNoder(),
		blockProposer: make(map[uint32][]byte),
		txnCollector:  txnCollector,
		msgChan:       make(chan interface{}, MsgChanCap),
		syncCache:     NewSyncBlockCache(),
		voting: []voting.Voting{
			voting.NewBlockVoting(totalWeight),
			voting.NewSigChainVoting(totalWeight, txnCollector),
		},
	}
	if !service.timer.Stop() {
		<-service.timer.C
	}
	if !service.timeout.Stop() {
		<-service.timeout.C
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

func (ps *ProposerService) ConsensusRoutine(vType voting.VotingContentType) {
	// initialization for height and voting weight
	ps.Initialize(vType, GetTotalVotingWeight(ps.localNode))
	current := ps.CurrentVoting(vType)
	votingHeight := current.GetVotingHeight()
	votingPool := current.GetVotingPool()

	go func() {
		// waiting for flooding finished
		time.Sleep(WaitingForFloodingFinished)
		// send new proposal
		err := ps.SendNewProposal(votingHeight, vType)
		if err != nil {
			log.Info("waiting for receiving proposed entity...")
			return
		}
	}()

	go func() {
		// waiting for voting finished
		time.Sleep(WaitingForVotingFinished)
		finalHash, ok := votingPool.GetMind(votingHeight)
		if !ok {
			return
		}
		// get the final entity from local cache or database
		content, err := current.GetVotingContent(finalHash, votingHeight)
		if err != nil {
			log.Errorf("get final entity error, hash: %s, type: %d, votingHeight: %d",
				BytesToHexString(finalHash.ToArrayReverse()), vType, votingHeight)
			log.Warn(err)
			return
		}
		// process final block and signature chain
		switch vType {
		case voting.SigChainTxnVote:
			txn := content.(*transaction.Transaction)
			payload := txn.Payload.(*payload.Commit)
			sigchain := &por.SigChain{}
			proto.Unmarshal(payload.SigChain, sigchain)
			// TODO: get a determinate public key on signature chain
			pbk, err := sigchain.GetLedgerNodePubkey()
			if err != nil {
				log.Warn("Get last public key error", err)
				return
			}
			ps.Lock()
			ps.blockProposer[votingHeight] = pbk
			ps.Unlock()
			sigChainTxnHash := content.Hash()
			log.Infof("sigchain transaction consensus: %s, %s will be block proposer for height %d",
				BytesToHexString(sigChainTxnHash.ToArrayReverse()), BytesToHexString(pbk), votingHeight)
		case voting.BlockVote:
			if block, ok := content.(*ledger.Block); ok {
				err = ledger.DefaultLedger.Blockchain.AddBlock(block)
				if err != nil {
					log.Error("saving block error: ", err)
					return
				}
			}
		}
	}()
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

func (ps *ProposerService) SendNewProposal(votingHeight uint32, vType voting.VotingContentType) error {
	current := ps.CurrentVoting(vType)
	content, err := current.GetBestVotingContent(votingHeight)
	if err != nil {
		return err
	}
	hash := content.Hash()
	votingPool := current.GetVotingPool()
	log.Info("local mind: ", BytesToHexString(hash.ToArrayReverse()))
	if mind, ok := votingPool.GetMind(votingHeight); ok {
		// if local mind doesn't better than neighbor mind then change.
		if mind.CompareTo(hash) == -1 {
			hash = mind
			log.Info("mind set to neighbor mind: ", BytesToHexString(hash.ToArrayReverse()))
		}
	} else {
		// set mind if it has not been set
		votingPool.ChangeMind(votingHeight, hash)
		log.Info("mind set to local mind: ", BytesToHexString(hash.ToArrayReverse()))
	}
	if !current.HasSelfState(hash, voting.ProposalSent) {
		log.Infof("proposing hash: %s, type: %d", BytesToHexString(hash.ToArrayReverse()), vType)
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

	return nil
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
	err = current.AddToCache(block)
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
	blockHash := block.Hash()
	votingPool.ChangeMind(votingHeight, blockHash)
	log.Info("when produce new block mind set to: ", BytesToHexString(blockHash.ToArrayReverse()))
}

func (ps *ProposerService) IsBlockProposer() bool {
	var err error
	var proposer []byte
	localPublicKey, err := ps.account.PublicKey.EncodePoint(true)
	if err != nil {
		return false
	}
	current := ps.CurrentVoting(voting.BlockVote)
	votingHeight := current.GetVotingHeight()
	if v, ok := ps.blockProposer[votingHeight]; ok {
		proposer = v
	} else {
		if len(config.Parameters.GenesisBlockProposer) < 1 {
			log.Warn("no GenesisBlockProposer configured")
			return false
		}
		proposer, err = HexStringToBytes(config.Parameters.GenesisBlockProposer[0])
		if err != nil || len(proposer) != crypto.COMPRESSEDLEN {
			log.Error("invalid GenesisBlockProposer configured")
			return false
		}
	}
	if !IsEqualBytes(localPublicKey, proposer) {
		return false
	}

	return true
}

func (ps *ProposerService) ProposerRoutine() {
	for {
		select {
		case <-ps.timer.C:
			if ps.IsBlockProposer() {
				log.Info("-> I am Block Proposer")
				ps.ProduceNewBlock()
				for _, v := range ps.voting {
					go ps.ConsensusRoutine(v.VotingType())
				}
				ps.timer.Reset(ConsensusTime)
			}
		}
	}
}

func (ps *ProposerService) TimeoutRoutine() {
	for {
		select {
		case <-ps.timeout.C:
			ps.timer.Stop()
			ps.timer.Reset(0)
		}
	}
}

func (ps *ProposerService) ProbeRoutine() {
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
}

func (ps *ProposerService) BlockPersistCompleted(v interface{}) {
	if block, ok := v.(*ledger.Block); ok {
		ps.txnCollector.Cleanup(block.Transactions)
	}
}

func (ps *ProposerService) BlockSyncingFinished(v interface{}) {
	var err error
	block := ps.syncCache.GetBlockFromSyncCache()
	for block != nil {
		err = ledger.BlockFullyCheck(block, ledger.DefaultLedger)
		if err != nil {
			log.Error("verifying cached block error: ", err)
		}
		err = ledger.DefaultLedger.Blockchain.AddBlock(block)
		if err != nil {
			log.Error("saving cached block error: ", err)
		}
		ps.syncCache.RemoveBlockFromCache()
		block = ps.syncCache.GetBlockFromSyncCache()
		// Fixme: wait for block persisted
		time.Sleep(time.Millisecond * 300)
	}
	log.Info("cached block saving finished")
	// switch syncing state
	ps.localNode.SetSyncState(protocol.PersistFinished)
}

func (ps *ProposerService) Start() error {
	// register consensus message
	ps.consensusMsgReceived = ps.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived,
		ps.ReceiveConsensusMsg)
	// register block saving event
	ps.blockPersisted = ledger.DefaultLedger.Blockchain.BCEvents.Subscribe(events.EventBlockPersistCompleted,
		ps.BlockPersistCompleted)
	// register block syncing event
	ps.syncFinished = ps.localNode.GetEvent("sync").Subscribe(events.EventBlockSyncingFinished,
		ps.BlockSyncingFinished)
	// start block syncing
	go ps.localNode.SyncBlock()
	// start monitor routine for block syncing
	go ps.localNode.SyncBlockMonitor()
	// start block proposer routine
	go ps.ProposerRoutine()
	// start timeout routine
	go ps.TimeoutRoutine()
	// trigger block proposer routine
	if ps.IsBlockProposer() {
		ps.timer.Reset(0)
	}
	// start probe routine
	go ps.ProbeRoutine()

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

func (ps *ProposerService) CreateCoinbaseTransaction() *transaction.Transaction {
	return &transaction.Transaction{
		TxType:         transaction.Coinbase,
		PayloadVersion: 0,
		Payload:        &payload.Coinbase{},
		Attributes: []*transaction.TxnAttribute{
			{
				Usage: transaction.Nonce,
				Data:  util.RandomBytes(transaction.TransactionNonceLength),
			},
		},
		Inputs: []*transaction.TxnInput{},
		Outputs: []*transaction.TxnOutput{
			{
				AssetID:     ledger.DefaultLedger.Blockchain.AssetID,
				Value:       10 * StorageFactor,
				ProgramHash: ps.account.ProgramHash,
			},
		},
		Programs: []*program.Program{},
	}
}

func (ps *ProposerService) BuildBlock() (*ledger.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []Uint256
	coinbase := ps.CreateCoinbaseTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	txns := ps.txnCollector.Collect()
	for txnHash, txn := range txns {
		if !ledger.DefaultLedger.Store.IsTxHashDuplicate(txnHash) {
			txnList = append(txnList, txn)
			txnHashList = append(txnHashList, txnHash)
		}
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

	// returns if receive duplicate block
	if current.HasSelfState(blockHash, voting.FloodingFinished) {
		log.Warn("Duplicate block received for hash: ", BytesToHexString(blockHash.ToArrayReverse()))
		return
	}
	// set state for flooding block
	current.SetSelfState(blockHash, voting.FloodingFinished)

	// if block syncing is not finished, cache received blocks in order
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		// set syncing stop block hash
		ps.localNode.SetSyncStopHash(bfMsg.block.Header.PrevBlockHash, height-1)
		err := ps.syncCache.AddBlockToSyncCache(bfMsg.block)
		if err != nil {
			log.Error("add received block to sync cache error: ", err)
		}
		log.Debugf("cached block: %s, block height: %d,  totally cached: %d",
			BytesToHexString(blockHash.ToArrayReverse()), height, ps.syncCache.CachedBlockHeight())
		return
	}

	// expect the height of received block is equal to voting height when block syncing finished
	if height != votingHeight {
		log.Warnf("receive block which height is invalid, consensus height: %d, received block height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(blockHash.ToArrayReverse()))
		return
	}
	// TODO check if the sender is PoR node
	err := current.AddToCache(bfMsg.block)
	if err != nil {
		log.Error("add received block to local cache error")
		return
	}
	// trigger consensus when receive appropriate block
	if !ps.IsBlockProposer() {
		for _, v := range ps.voting {
			go ps.ConsensusRoutine(v.VotingType())
		}
		// trigger block proposer changed when tolerance time not receive block
		ps.timeout.Reset(ConsensusTime + TimeoutTolerance)
	}
}

func (ps *ProposerService) HandleRequestMsg(req *Request, sender *crypto.PubKey) {
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		return
	}
	current := ps.CurrentVoting(req.contentType)
	votingHeight := current.GetVotingHeight()
	hash := *req.hash
	height := req.height
	if height < votingHeight {
		log.Warnf("receive invalid request, consensus height: %d, request height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// TODO check if already sent Proposal to sender
	if hash.CompareTo(current.GetConfirmingHash()) != 0 {
		log.Warn("requested block doesn't match with local block in process")
		return
	}
	if !current.HasSelfState(hash, voting.ProposalSent) {
		log.Warn("receive invalid request for hash: ", BytesToHexString(hash.ToArrayReverse()))
		return
	}
	nodeID := publickKeyToNodeID(sender)
	// returns if receive duplicate request
	if current.HasNeighborState(nodeID, hash, voting.RequestReceived) {
		log.Warn("duplicate request received for hash: ", BytesToHexString(hash.ToArrayReverse()))
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
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		return
	}
	votingType := resp.contentType
	current := ps.CurrentVoting(votingType)
	votingHeight := current.GetVotingHeight()
	hash := resp.hash
	height := resp.height
	if height != votingHeight {
		log.Warnf("receive invalid response, consensus height: %d, response height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
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
		log.Warn("duplicate response received for hash: ", BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// TODO check if the sender is requested neighbor node
	err := current.AddToCache(resp.content)
	if err != nil {
		return
	}
	currentVotingPool := current.GetVotingPool()
	currentVotingPool.AddToReceivePool(height, nodeID, *hash)
	current.SetNeighborState(nodeID, *hash, voting.ProposalReceived)

	// Get voting result from voting pool. If votes is not enough then return.
	maybeFinalHash, err := currentVotingPool.VoteCounting(votingHeight)
	if err != nil {
		return
	}
	ps.SetOrChangeMind(votingType, votingHeight, maybeFinalHash)
}

func (ps *ProposerService) SetOrChangeMind(votingType voting.VotingContentType,
	votingHeight uint32, maybeFinalHash *Uint256) {
	current := ps.CurrentVoting(votingType)
	currentVotingPool := current.GetVotingPool()
	if mind, ok := currentVotingPool.GetMind(votingHeight); ok {
		// When current mind has been set, if voting result is different with
		// current mind then do mind changing.
		if mind.CompareTo(*maybeFinalHash) != 0 {
			log.Info("when receive proposal mind changed to neighbor mind: ",
				BytesToHexString(maybeFinalHash.ToArrayReverse()))
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
		log.Info("when receive proposal mind set to neighbor mind: ",
			BytesToHexString(maybeFinalHash.ToArrayReverse()))
	}
}

func (ps *ProposerService) HandleProposalMsg(proposal *Proposal, sender *crypto.PubKey) {
	// TODO: vote counting for cached blocks
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		return
	}
	current := ps.CurrentVoting(proposal.contentType)
	votingType := current.VotingType()
	votingHeight := current.GetVotingHeight()
	hash := *proposal.hash
	height := proposal.height
	if height < votingHeight-1 {
		log.Warnf("receive invalid proposal, consensus height: %d, proposal height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
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
			BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// returns if receive duplicated proposal
	if current.HasNeighborState(nodeID, hash, voting.ProposalReceived) {
		log.Warn("duplicate proposal received for hash: ", BytesToHexString(hash.ToArrayReverse()))
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
	ps.SetOrChangeMind(votingType, votingHeight, maybeFinalHash)
}

func (ps *ProposerService) HandleMindChangingMsg(mindChanging *MindChanging, sender *crypto.PubKey) {
	// TODO: vote counting for cached blocks
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		return
	}
	current := ps.CurrentVoting(mindChanging.contentType)
	votingType := current.VotingType()
	votingHeight := current.GetVotingHeight()
	hash := *mindChanging.hash
	height := mindChanging.height
	if height < votingHeight {
		log.Warnf("receive invalid mind changing, consensus height: %d, mind changing height: %d,"+
			" hash: %s\n", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
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
			log.Info("when receive mindchanging mind change to neighbor mind: ",
				BytesToHexString(maybeFinalHash.ToArrayReverse()))
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
