package ising

import (
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/ising/voting"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/events"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

const (
	TxnAmountToBePackaged      = 20480
	WaitingForFloodingFinished = time.Second * 6
	WaitingForVotingFinished   = time.Second * 16
	TimeoutTolerance           = time.Second * 2
	ForkingDetectTimer         = time.Second * 5
	WaitingForProbeFinished    = time.Second * 3
	BlockRollbackStep          = 5
)

type ProposerService struct {
	sync.RWMutex
	account              *vault.Account            // local account
	timer                *time.Timer               // timer for proposer node
	timeout              *time.Timer               // timeout for next round consensus
	proposerChangeTimer  *time.Timer               // timer for proposer change
	localNode            protocol.Noder            // local node
	txnCollector         *transaction.TxnCollector // collect transaction from where
	mining               Mining                    // built-in mining
	consensusMsgReceived events.Subscriber         // consensus events listening
	blockPersisted       events.Subscriber         // block saved events
	blockPersistedLock   sync.RWMutex              // block persisted lock
	syncFinished         events.Subscriber         // block syncing finished event
	forkCache            *ForkCache                // cache for handling block forking
	proposerCache        *ProposerCache            // cache for block proposer
	syncCache            *SyncCache                // cache for block syncing
	voting               []voting.Voting           // array for sigchain and block voting
}

func NewProposerService(account *vault.Account, node protocol.Noder) *ProposerService {
	txnCollector := transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged)

	service := &ProposerService{
		timer:               time.NewTimer(config.ConsensusTime),
		timeout:             time.NewTimer(config.ConsensusTime + TimeoutTolerance),
		proposerChangeTimer: time.NewTimer(config.ProposerChangeTime),
		account:             account,
		localNode:           node,
		txnCollector:        txnCollector,
		mining:              NewBuiltinMining(account, txnCollector),
		syncCache:           NewSyncBlockCache(),
		proposerCache:       NewProposerCache(),
		voting: []voting.Voting{
			voting.NewBlockVoting(),
			voting.NewSigChainVoting(txnCollector),
		},
	}
	if !service.timer.Stop() {
		<-service.timer.C
	}
	if !service.timeout.Stop() {
		<-service.timeout.C
	}

	go service.HandleBlockForking()

	return service
}

// FilterNoderByIDs filter out a node slice who its ID was specified in 'nids'
func FilterNoderByIDs(nodes []protocol.Noder, nids []uint64) (ret []protocol.Noder) {
	if nids == nil {
		return nodes
	}
	for _, id := range nids {
		for _, n := range nodes {
			if id == n.GetID() {
				ret = append(ret, n)
			}
		}
	}
	return ret
}

func (ps *ProposerService) ProcessRollback() {
	forkedPointFound := false
	genesisChecked := false
	ps.forkCache.probeHeight = ledger.DefaultLedger.Store.GetHeight()

	for !forkedPointFound {
		if genesisChecked && 0 == ps.forkCache.probeHeight {
			log.Error("genesis block different. local database error")
			ps.forkCache.SetRollBackHeight(0)
			break
		}

		reqMsg := NewPing(ps.forkCache.probeHeight, ps.localNode.GetSyncState())
		nodes := ps.GetPersistedNode(nil)
		ps.SendConsensusMsg(reqMsg, nodes)
		time.Sleep(WaitingForProbeFinished)
		forkedPointFound = ps.forkCache.AnalyzeProbeResp()

		if ps.forkCache.probeHeight < BlockRollbackStep {
			ps.forkCache.probeHeight = 0
			genesisChecked = true
		} else {
			ps.forkCache.probeHeight -= BlockRollbackStep
		}
	}
	if ps.forkCache.GetRollBackHeight() == 0 {
		log.Error("local database error, need to check genesis block")
	} else {
		cHeight := int(ps.forkCache.GetCurrentHeight())
		rHeight := int(ps.forkCache.GetRollBackHeight())
		log.Warningf("roll back blocks from height %d to %d", cHeight, rHeight)
		// TODO: Lock db before Rollback for avoid corrupted ledger
		for i := cHeight; i > rHeight; i-- {
			b, err := ledger.DefaultLedger.Store.GetBlockByHeight(uint32(i))
			if err != nil {
				log.Errorf("get block error when roll back which height is %d", i)
			}
			err = ledger.DefaultLedger.Store.Rollback(b)
			if err != nil {
				log.Error("roll back error: ", err)
			}
		}
	}
	// exit after rolling back block
	log.Error("Forking detected, prepare to rollback")
	panic("Forking detected, prepare to rollback")
}

func (ps *ProposerService) HandleBlockForking() {
	timer := time.NewTimer(ForkingDetectTimer)
	shouldAnalyzePingResp := false
	for {
		select {
		case <-timer.C:
			if ps.localNode.GetSyncState() == protocol.PersistFinished {
				shouldAnalyzePingResp = true
			}

			//send ping to all neighbors periodically
			nodes := ps.GetReceiverNode(nil)
			currentHeight := ledger.DefaultLedger.Store.GetHeight()
			currentHash := ledger.DefaultLedger.Store.GetCurrentBlockHash()
			ps.forkCache = NewForkCache(currentHeight, currentHash)
			// ping neighbors with local current height
			pingMsg := NewPing(MaxUint32, ps.localNode.GetSyncState()) // ping remote with MaxUint32 for get latest hash
			ps.SendConsensusMsg(pingMsg, nodes)

			// wait response from neighbor and handle rollback if needed
			time.Sleep(WaitingForProbeFinished)

			// Don't check forked during block syncing
			if shouldAnalyzePingResp && ps.forkCache.AnalyzePingResp() {
				ps.ProcessRollback()
			}

			// reset timer if no forking
			timer.Reset(ForkingDetectTimer)
		}
	}
}

func (ps *ProposerService) CurrentVoting(vType voting.VotingContentType) voting.Voting {
	for _, v := range ps.voting {
		if v.VotingType() == vType {
			return v
		}
	}

	return nil
}

func (ps *ProposerService) ConsensusRoutine(vType voting.VotingContentType, isProposer bool) {
	// initialization for height and voting weight
	ps.Initialize(vType)
	current := ps.CurrentVoting(vType)
	votingHeight := current.GetVotingHeight()
	votingPool := current.GetVotingPool()

	go func() {
		// waiting for flooding finished
		time.Sleep(WaitingForFloodingFinished)
		// send new proposal
		err := ps.SendNewProposal(votingHeight, vType, isProposer)
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
			log.Warning(err)
			return
		}
		// process final block and signature chain
		switch vType {
		case voting.SigChainTxnVote:
			if txn, ok := content.(*transaction.Transaction); ok {
				ps.proposerCache.Add(votingHeight, txn)
			}
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
	return FilterNoderByIDs(ps.localNode.GetNeighborNoder(), nids)
}

// GetPersistedNode returns PersistFinished nodes from neighbors according to neighbor node ID passed in.
// If 'nids' passed in is nil then returns all.
func (ps *ProposerService) GetPersistedNode(nids []uint64) []protocol.Noder {
	return FilterNoderByIDs(ps.localNode.GetSyncFinishedNeighbors(), nids)
}

func (ps *ProposerService) SendNewProposal(votingHeight uint32, vType voting.VotingContentType, isProposer bool) error {
	current := ps.CurrentVoting(vType)
	content, err := current.GetBestVotingContent(votingHeight)
	if err != nil {
		return err
	}
	if !isProposer && !current.VerifyVotingContent(content) {
		return errors.New("verify voting content error when send new proposal")
	}
	votingPool := current.GetVotingPool()
	ownMindHash := content.Hash()
	ownWeight, _ := ledger.DefaultLedger.Store.GetVotingWeight(Uint160{})
	ownNodeID := ps.localNode.GetID()
	log.Infof("own mind: %s, type: %d", BytesToHexString(ownMindHash.ToArrayReverse()), vType)
	if mind, ok := votingPool.GetMind(votingHeight); ok {
		log.Infof("neighbor mind: %s, type: %d ", BytesToHexString(mind.ToArrayReverse()), vType)
		// if own mind different with neighbors then change mind
		if ownMindHash.CompareTo(mind) != 0 {
			ownMindHash = mind
			log.Infof("own mind is affected by neighbor mind: %s, type: %d",
				BytesToHexString(ownMindHash.ToArrayReverse()), vType)
		}
		// add own vote to voting pool
		votingPool.AddToReceivePool(votingHeight, ownNodeID, ownWeight, ownMindHash)
	} else {
		// add self mind to voting pool, if votes is enough then set mind.
		maybeFinalHash, _ := votingPool.AddVoteThenCounting(votingHeight, ownNodeID, ownWeight, ownMindHash)
		if maybeFinalHash != nil {
			log.Infof("mind set when send proposal: %s, type: %d",
				BytesToHexString(ownMindHash.ToArrayReverse()), vType)
			votingPool.SetMind(votingHeight, ownMindHash)
		}
	}
	if !current.CheckAndSetOwnState(ownMindHash, voting.ProposalSent) {
		log.Infof("proposing hash: %s, type: %d", BytesToHexString(ownMindHash.ToArrayReverse()), vType)
		// create new proposal
		proposalMsg := NewProposal(&ownMindHash, votingHeight, vType)
		// get nodes which should receive proposal message
		nodes := ps.GetReceiverNode(nil)
		// send proposal to neighbors
		ps.SendConsensusMsg(proposalMsg, nodes)
		// set confirming hash
		current.SetConfirmingHash(ownMindHash)
	}

	return nil
}

func (ps *ProposerService) ProduceNewBlock() {
	current := ps.CurrentVoting(voting.BlockVote)
	votingHeight := current.GetVotingHeight()
	info := ps.proposerCache.Get(votingHeight + 1)
	if info == nil {
		info = &ProposerInfo{
			winnerHash: EmptyUint256,
			winnerType: ledger.BlockSigner,
		}
	}
	// build new block to be proposed
	block, err := ps.mining.BuildBlock(votingHeight, ps.localNode.GetChordAddr(),
		info.winnerHash, info.winnerType)
	if err != nil {
		log.Error("building block error: ", err)
	}
	err = current.AddToCache(block, time.Now().Unix())
	if err != nil {
		log.Error("adding block to cache error: ", err)
		return
	}
	err = ledger.TransactionCheck(block)
	if err != nil {
		log.Error("found invalide transaction when produce new block")
		return
	}
	// generate BlockFlooding message
	blockFlooding := NewBlockFlooding(block)
	// send BlockFlooding message
	err = ps.BroadcastConsensusMsg(blockFlooding)
	if err != nil {
		log.Error("sending consensus message error: ", err)
	}
	blockHash := block.Hash()
	log.Info("produce new block: ", BytesToHexString(blockHash.ToArrayReverse()))
}

func (ps *ProposerService) IsBlockProposer() bool {
	localPublicKey, err := ps.account.PublicKey.EncodePoint(true)
	if err != nil {
		return false
	}
	current := ps.CurrentVoting(voting.BlockVote)
	votingHeight := current.GetVotingHeight()
	proposerInfo := ps.proposerCache.Get(votingHeight)
	if proposerInfo == nil {
		return false
	}
	if !IsEqualBytes(localPublicKey, proposerInfo.publicKey) {
		return false
	}
	if len(proposerInfo.chordID) != 0 && !IsEqualBytes(ps.localNode.GetChordAddr(), proposerInfo.chordID) {
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
				if ps.localNode.GetSyncState() != protocol.PersistFinished {
					ps.localNode.StopSyncBlock(true)
				}
				ps.ProduceNewBlock()
				for _, v := range ps.voting {
					go ps.ConsensusRoutine(v.VotingType(), true)
				}
				ps.timer.Reset(config.ConsensusTime)
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

func (ps *ProposerService) BlockPersistCompleted(v interface{}) {
	ps.blockPersistedLock.Lock()
	defer ps.blockPersistedLock.Unlock()

	// reset timer when block persisted
	ps.proposerChangeTimer.Stop()
	ps.proposerChangeTimer.Reset(config.ProposerChangeTime)

	if ps.localNode.GetSyncState() == protocol.PersistFinished {
		if block, ok := v.(*ledger.Block); ok {
			// record time when persist block
			ledger.DefaultLedger.Blockchain.AddBlockTime(block.Hash(), time.Now().Unix())
			ps.txnCollector.Cleanup(block.Transactions)
		}
	}
}

func (ps *ProposerService) ChangeProposerRoutine() {
	for {
		select {
		case <-ps.proposerChangeTimer.C:
			// now := time.Now().Unix()
			currentHeight := ledger.DefaultLedger.Store.GetHeight()
			var block *ledger.Block
			var err error
			if currentHeight < InitialBlockHeight {
				block, err = ledger.DefaultLedger.Store.GetBlockByHeight(0)
				if err != nil {
					log.Error("get genesis block error when change proposer")
				}
			} else {
				// currentBlock, err := ledger.DefaultLedger.Store.GetBlockByHeight(currentHeight)
				if err != nil {
					log.Errorf("get latest block %d error when change proposer", currentHeight)
				}
				// This is a temporary solution
				var height uint32 = 0
				// timestamp := currentBlock.Header.Timestamp
				// index := (now - timestamp) / int64(config.ProposerChangeTime/time.Second)
				// if int64(currentHeight) > index {
				// height = uint32(int64(currentHeight) - index)
				// }
				block, err = ledger.DefaultLedger.Store.GetBlockByHeight(height)
				if err != nil {
					log.Errorf("get block %d error when change proposer", currentHeight)
				}
			}
			ps.proposerCache.Add(currentHeight+1, block)
			ps.timer.Stop()
			ps.timer.Reset(0)
			ps.proposerChangeTimer.Reset(config.ProposerChangeTime)
		}
	}
}

func (ps *ProposerService) PersistCachedBlock(height uint32) error {
	//TODO: re-sync block if the block can not be persisted
	vBlock, err := ps.syncCache.WaitBlockVotingFinished(height)
	if err != nil {
		return err
	}
	err = ledger.HeaderCheck(vBlock.Block.Header, vBlock.ReceiveTime)
	if err != nil {
		return err
	}
	err = ledger.TransactionCheck(vBlock.Block)
	if err != nil {
		return err
	}
	err = ledger.DefaultLedger.Blockchain.AddBlock(vBlock.Block)
	if err != nil {
		return err
	}
	// Fixme: wait for block persisted
	time.Sleep(time.Millisecond * 300)

	return nil
}

func (ps *ProposerService) BlockSyncingFinished(v interface{}) {
	skip, ok := v.(bool)
	if !ok {
		log.Error("got invalid notice from stopping sync event")
		return
	}
	if skip {
		log.Info("skip saving cached blocks")
		// skip persisting cached blocks when skip is true
		ps.localNode.SetSyncState(protocol.PersistFinished)
		return
	}

	var err error
	if ps.syncCache.consensusHeight == ledger.DefaultLedger.Store.GetHeight()+1 {
		log.Infof("start saving cached blocks right away, consensus height: %d", ps.syncCache.consensusHeight)
		err = ps.PersistCachedBlock(ps.syncCache.consensusHeight)
		if err != nil {
			// Workaround. Don't hung at SyncFinished state
			err = fmt.Errorf("persist cached block error: %v, height: %d", err, ps.syncCache.consensusHeight)
			log.Error(err)
			panic(err)
		}
		for i := ps.syncCache.minHeight; i <= ps.syncCache.maxHeight; i++ {
			// cleanup cached block
			err = ps.syncCache.RemoveBlockFromCache(i)
			if err != nil {
				log.Warningf("sync cache cleanup failed for height %d, error: %v", i, err)
			}
			// cleanup block time lock
			ps.syncCache.timeLock.RemoveForHeight(i)
		}
		log.Info("cached block saving finished")
		// switch syncing state
		ps.localNode.SetSyncState(protocol.PersistFinished)
		return
	}

	log.Infof("process cached blocks, from height: %d, to height: %d, consensus height: %d, start height: %d",
		ps.syncCache.minHeight, ps.syncCache.maxHeight,
		ps.syncCache.consensusHeight, ps.syncCache.consensusHeight+1)
	for i := ps.syncCache.minHeight; i <= ps.syncCache.maxHeight; i++ {
		if i > ps.syncCache.consensusHeight {
			err = ps.PersistCachedBlock(i)
			if err != nil {
				// Workaround. Don't hung at SyncFinished state
				err = fmt.Errorf("persist cached block error: %v, height: %d", err, i)
				log.Error(err)
				panic(err)
			}
		}
		// cleanup cached block
		err = ps.syncCache.RemoveBlockFromCache(i)
		if err != nil {
			log.Warningf("sync cache cleanup failed for height %d, error: %v", i, err)
		}
		// cleanup block time lock
		ps.syncCache.timeLock.RemoveForHeight(i)
	}
	log.Info("cached block saving finished")
	// switch syncing state
	ps.localNode.SetSyncState(protocol.PersistFinished)
}
func (ps *ProposerService) SyncBlock(isProposer bool) {
	if ps.localNode.GetSyncState() == protocol.PersistFinished ||
		ps.localNode.GetSyncState() == protocol.SyncFinished {
		return
	}
	var wg sync.WaitGroup
	wg.Add(1)
	// start block syncing
	go func() {
		defer wg.Done()
		ps.localNode.SyncBlock(isProposer)
	}()
	wg.Add(1)
	// start monitor routine for block syncing
	go func() {
		defer wg.Done()
		ps.localNode.SyncBlockMonitor(isProposer)
	}()
	wg.Wait()
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

	// start block proposer routine
	go ps.ProposerRoutine()
	// start change proposer routine
	go ps.ChangeProposerRoutine()
	// start timeout routine
	go ps.TimeoutRoutine()

	ps.SyncBlock(false)

	return nil
}

func (ps *ProposerService) newConsensusMessage(msg IsingMessage) ([]byte, error) {
	isingPld, err := BuildIsingPayload(msg, ps.account.PublicKey)
	if err != nil {
		return nil, err
	}
	hash, err := isingPld.DataHash()
	if err != nil {
		return nil, err
	}
	signature, err := crypto.Sign(ps.account.PrivateKey, hash)
	if err != nil {
		return nil, err
	}
	isingPld.Signature = signature

	buf, err := message.NewIsingConsensus(isingPld)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (ps *ProposerService) SendConsensusMsg(msg IsingMessage, to []protocol.Noder) error {
	if len(to) == 0 {
		return nil
	}

	buf, err := ps.newConsensusMessage(msg)
	if err != nil {
		return err
	}

	for _, node := range to {
		node.Tx(buf)
	}

	return nil
}

func (ps *ProposerService) BroadcastConsensusMsg(msg IsingMessage) error {
	buf, err := ps.newConsensusMessage(msg)
	if err != nil {
		return err
	}

	return ps.localNode.Broadcast(buf)
}

func (ps *ProposerService) ReceiveConsensusMsg(v interface{}) {
	if info, ok := v.(*message.NotifyInfo); ok {
		senderPubKey := info.Payload.Sender
		sender := info.SenderID
		signature := info.Payload.Signature
		hash, err := info.Payload.DataHash()
		if err != nil {
			fmt.Println("get consensus payload hash error")
			return
		}
		err = crypto.Verify(*senderPubKey, hash, signature)
		if err != nil {
			fmt.Println("consensus message verification error")
			return
		}
		isingMsg, err := RecoverFromIsingPayload(info.Payload)
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
		case *Proposal:
			ps.HandleProposalMsg(t, sender)
		case *MindChanging:
			ps.HandleMindChangingMsg(t, sender)
		case *Ping:
			ps.HandlePingMsg(t, sender)
		case *Pong:
			ps.HandlePongMsg(t, sender)
		default:
			log.Error("Unknown Ising message")
		}

	}
}

func (ps *ProposerService) HandleBlockFloodingMsg(bfMsg *BlockFlooding, sender uint64) {
	current := ps.CurrentVoting(voting.BlockVote)
	votingHeight := current.GetVotingHeight()
	block := bfMsg.block
	blockHash := bfMsg.block.Hash()
	height := bfMsg.block.Header.Height
	rtime := time.Now().Unix()

	// returns if receive duplicate block
	if current.CheckAndSetOwnState(blockHash, voting.FloodingFinished) {
		return
	}

	// if block syncing is not finished, cache received blocks
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		err := ps.syncCache.AddBlockToSyncCache(block, rtime)
		if err != nil {
			log.Error("add received block to sync cache error: ", err)
		}
		log.Infof("cached block: %s, block height: %d,  totally cached: %d",
			BytesToHexString(blockHash.ToArrayReverse()), height, ps.syncCache.CachedBlockHeight())
		if !HasAbilityToVerifyBlock(block) {
			return
		}
		// send vote when the block is verified by local node
		if !current.CheckAndSetOwnState(blockHash, voting.ProposalSent) {
			err = ledger.HeaderCheck(block.Header, rtime)
			if err != nil {
				log.Error("header verification error when voting in sync mode", err)
				return
			}

			err = ledger.TimestampCheck(block.Header.Timestamp)
			if err != nil {
				log.Error("Proposal fails to pass timestamp check", err)
				return
			}

			err = ledger.TransactionCheck(block)
			if err != nil {
				log.Error("transaction verification error when voting in sync mode", err)
				return
			}

			log.Infof("send vote to block %s while syncing", BytesToHexString(blockHash.ToArrayReverse()))
			proposalMsg := NewProposal(&blockHash, votingHeight, voting.BlockVote)
			nodes := ps.GetReceiverNode(nil)
			ps.SendConsensusMsg(proposalMsg, nodes)
		}
		return
	}

	// expect the height of received block is equal to voting height when block syncing finished
	if height != votingHeight {
		log.Warningf("receive block which height is invalid, consensus height: %d, received block height: %d,"+
			" hash: %s", votingHeight, height, BytesToHexString(blockHash.ToArrayReverse()))
		return
	}
	err := current.AddToCache(block, rtime)
	if err != nil {
		log.Error("add received block to local cache error: ", err)
		return
	}

	// trigger consensus when receive appropriate block
	if !ps.IsBlockProposer() {
		for _, v := range ps.voting {
			go ps.ConsensusRoutine(v.VotingType(), false)
		}
		// trigger block proposer changed when tolerance time not receive block
		ps.timeout.Reset(config.ConsensusTime + TimeoutTolerance)
	}
}

func (ps *ProposerService) HandleRequestMsg(req *Request, sender uint64) {
	current := ps.CurrentVoting(req.contentType)
	votingType := current.VotingType()
	votingHeight := current.GetVotingHeight()
	hash := *req.hash
	height := req.height

	if height < votingHeight {
		log.Warningf("receive invalid request, consensus height: %d, request height: %d,"+
			" hash: %s", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// returns if never send vote
	if !current.CheckOwnState(hash, voting.ProposalSent) {
		log.Warning("receive invalid request for hash: ", BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// returns if receive duplicate request
	if current.CheckAndSetNeighborState(sender, hash, voting.RequestReceived) {
		log.Warning("duplicate request received for hash: ", BytesToHexString(hash.ToArrayReverse()))
		return
	}

	var content voting.VotingContent
	var err error
	// get block from sync cache when in sync state, get block and transaction
	// from consensus cache when in consensus state.
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		if votingType != voting.BlockVote {
			return
		}
		content, err = ps.syncCache.GetBlock(req.height, req.hash)
		if err != nil {
			log.Error(err)
			return
		}
	} else {
		if hash.CompareTo(current.GetConfirmingHash()) != 0 {
			log.Warning("requested block doesn't match with local block in process")
			return
		}
		content, err = current.GetVotingContent(hash, height)
		if err != nil {
			return
		}
	}

	// generate response message
	responseMsg := NewResponse(&hash, height, votingType, content)
	// get node which should receive response message
	nodes := ps.GetReceiverNode([]uint64{sender})
	// send response message
	ps.SendConsensusMsg(responseMsg, nodes)
}

func (ps *ProposerService) Initialize(vType voting.VotingContentType) {
	// initial total voting weight
	for _, v := range ps.voting {
		v.GetVotingPool().Reset()
		v.Reset()
	}
}

func (ps *ProposerService) HandleResponseMsg(resp *Response, sender uint64) {
	votingType := resp.contentType
	current := ps.CurrentVoting(votingType)
	votingHeight := current.GetVotingHeight()
	hash := resp.hash
	height := resp.height

	// returns if not received proposal
	if !current.CheckNeighborState(sender, *hash, voting.ProposalReceived) {
		log.Warning("not receive proposal but receive response for hash: ",
			BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// returns if no request sent before
	if !current.CheckNeighborState(sender, *hash, voting.RequestSent) {
		log.Warning("consensus state error in Response message handler")
		return
	}
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		if votingType != voting.BlockVote {
			return
		}
		if b, ok := resp.content.(*ledger.Block); ok {
			ps.syncCache.AddBlockToSyncCache(b, time.Now().Unix())
		}
		return
	} else {
		if height != votingHeight {
			log.Warningf("receive invalid response, consensus height: %d, response height: %d,"+
				" hash: %s", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
			return
		}
		err := current.AddToCache(resp.content, time.Now().Unix())
		if err != nil {
			return
		}
		currentVotingPool := current.GetVotingPool()
		neighborWeight, _ := ledger.DefaultLedger.Store.GetVotingWeight(Uint160{})
		// Get voting result from voting pool. If votes is not enough then return.
		maybeFinalHash, err := currentVotingPool.AddVoteThenCounting(votingHeight, sender, neighborWeight, *hash)
		if err != nil {
			return
		}
		ps.SetOrChangeMind(votingType, votingHeight, maybeFinalHash)
	}
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
			history := currentVotingPool.SetMind(votingHeight, *maybeFinalHash)
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
		currentVotingPool.SetMind(votingHeight, *maybeFinalHash)
		log.Info("mind set when receive vote: ", BytesToHexString(maybeFinalHash.ToArrayReverse()))
	}
}

func (ps *ProposerService) HandleProposalMsg(proposal *Proposal, sender uint64) {
	hash := *proposal.hash
	height := proposal.height
	current := ps.CurrentVoting(proposal.contentType)
	votingType := current.VotingType()

	if current.CheckAndSetNeighborState(sender, hash, voting.ProposalReceived) {
		log.Warning("duplicate proposal received for hash: ", BytesToHexString(hash.ToArrayReverse()))
		return
	}
	// handle block proposal when block syncing
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		if votingType != voting.BlockVote {
			return
		}
		// Cache vote when in sync mode. If voted block doesn't exist in sync cache then request it from neighbor
		if exist := ps.syncCache.AddVoteForBlock(hash, height, sender); !exist {
			requestMsg := NewRequest(&hash, height, votingType)
			nodes := ps.GetReceiverNode([]uint64{sender})
			ps.SendConsensusMsg(requestMsg, nodes)
			current.CheckAndSetNeighborState(sender, hash, voting.RequestSent)
			log.Warningf("doesn't contain block hash in sync cache, requesting it from neighbor %s",
				BytesToHexString(hash.ToArrayReverse()))
		}
		// TODO: start timer when receive first
		time.Sleep(2 * time.Second)
		vBlock, err := ps.syncCache.WaitBlockVotingFinished(height)
		if err != nil {
			return
		}
		// if local node has ability to verify block then set stop hash to current block
		// to trigger syncing finished right away.
		if HasAbilityToVerifyBlock(vBlock.Block) {
			currentBlockHash := ledger.DefaultLedger.Store.GetCurrentBlockHash()
			currentBlockHeight := ledger.DefaultLedger.Store.GetHeight()
			ps.localNode.SetSyncStopHash(currentBlockHash, currentBlockHeight)
		} else {
			ps.localNode.SetSyncStopHash(vBlock.Block.Header.Hash(), vBlock.Block.Header.Height)
		}
		ps.syncCache.SetConsensusHeight(height)
		return
	}

	votingHeight := current.GetVotingHeight()
	neighbors := ps.localNode.GetNeighborNoder()
	if height < votingHeight {
		log.Warningf("receive invalid proposal, consensus height: %d, proposal height: %d,"+
			" hash: %s", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
		return
	}
	if height > votingHeight {
		neighborHeight, count := current.CacheProposal(height)
		if 2*count > len(neighbors) {
			err := fmt.Errorf("state is different with neighbors, "+
				"current voting height: %d, neighbor height: %d (%d/%d), exits.",
				votingHeight, neighborHeight, count, len(neighbors))
			log.Error(err)
			panic(err)
		}
		return
	}
	if !current.Exist(hash, height) {
		// generate request message
		requestMsg := NewRequest(&hash, height, votingType)
		// get node which should receive request message
		nodes := ps.GetReceiverNode([]uint64{sender})
		// send request message
		ps.SendConsensusMsg(requestMsg, nodes)
		current.CheckAndSetNeighborState(sender, hash, voting.RequestSent)
		log.Warningf("doesn't contain hash in local cache, requesting it from neighbor %s",
			BytesToHexString(hash.ToArrayReverse()))
		return
	}

	currentVotingPool := current.GetVotingPool()
	neighborWeight, _ := ledger.DefaultLedger.Store.GetVotingWeight(Uint160{})
	// Get voting result from voting pool. If votes is not enough then return.
	maybeFinalHash, err := currentVotingPool.AddVoteThenCounting(votingHeight, sender, neighborWeight, hash)
	if err != nil {
		return
	}
	ps.SetOrChangeMind(votingType, votingHeight, maybeFinalHash)
}

func (ps *ProposerService) HandleMindChangingMsg(mindChanging *MindChanging, sender uint64) {
	hash := *mindChanging.hash
	height := mindChanging.height

	// handle mind changing when block syncing
	if ps.localNode.GetSyncState() != protocol.PersistFinished {
		if mindChanging.contentType == voting.BlockVote {
			ps.syncCache.AddVoteForBlock(hash, height, sender)
		}
		return
	}

	current := ps.CurrentVoting(mindChanging.contentType)
	votingType := current.VotingType()
	votingHeight := current.GetVotingHeight()
	if height != votingHeight {
		log.Warningf("receive invalid mind changing, consensus height: %d, mind changing height: %d,"+
			" hash: %s", votingHeight, height, BytesToHexString(hash.ToArrayReverse()))
		return
	}
	currentVotingPool := current.GetVotingPool()
	if !currentVotingPool.HasReceivedVoteFrom(votingHeight, sender) {
		log.Warning("no proposal received before, so mind changing is invalid")
		return
	}
	neighborWeight, _ := ledger.DefaultLedger.Store.GetVotingWeight(Uint160{})
	// recalculate votes
	maybeFinalHash, err := currentVotingPool.AddVoteThenCounting(votingHeight, sender, neighborWeight, hash)
	if err != nil {
		return
	}
	if mind, ok := currentVotingPool.GetMind(votingHeight); ok {
		// When current mind has been set, if voting result is different with
		// current mind then do mind changing.
		if mind.CompareTo(*maybeFinalHash) != 0 {
			log.Info("when receive mindchanging mind change to neighbor mind: ",
				BytesToHexString(maybeFinalHash.ToArrayReverse()))
			history := currentVotingPool.SetMind(votingHeight, *maybeFinalHash)
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

func (ps *ProposerService) HandlePingMsg(msg *Ping, sender uint64) {
	var err error
	var hash Uint256
	var height = msg.height
	var pingHeight = msg.height

	hash, err = ledger.DefaultLedger.Store.GetBlockHash(msg.height)
	if err != nil {
		// when request height doesn't exist, return current height and hash
		height = ledger.DefaultLedger.Store.GetHeight()
		hash = ledger.DefaultLedger.Store.GetCurrentBlockHash()
	}
	pongMsg := NewPong(hash, height, pingHeight, ps.localNode.GetSyncState())
	nodes := ps.GetReceiverNode([]uint64{sender})
	for _, n := range nodes {
		n.SetHeight(pingHeight)
		n.SetSyncState(msg.syncState)
	}
	ps.SendConsensusMsg(pongMsg, nodes)
}

func (ps *ProposerService) HandlePongMsg(msg *Pong, sender uint64) {
	if ps.forkCache == nil {
		log.Error("no ping, receive pong.")
		return
	}
	if msg.syncState != protocol.PersistFinished {
		return
	}
	hash := msg.blockHash
	height := msg.blockHeight
	switch msg.pingHeight {
	case MaxUint32:
		// height delta 1 should seem as NOT forked.
		if 1 == AbsUint(uint(height), uint(ps.forkCache.currentHeight)) {
			// Use myCache hash & height instead of remote's, for ignore delta 1 difference
			hash, height = ps.forkCache.currentHash, ps.forkCache.currentHeight
		}
		err := ps.forkCache.CachePingResp(hash, height, sender)
		if err != nil {
			log.Error(err)
			return
		}
	case ps.forkCache.probeHeight:
		err := ps.forkCache.CacheProbeResp(hash, height, sender)
		if err != nil {
			log.Error(err)
			return
		}
	default:
		log.Error("invalid pong message")
		return
	}
}
