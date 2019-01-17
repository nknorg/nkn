package moca

import (
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/moca/election"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/vault"
)

// Consensus is the Majority vOte Cellular Automata (MOCA) consensus layer
type Consensus struct {
	account             *vault.Account
	localNode           *node.LocalNode
	startOnce           sync.Once
	elections           common.Cache
	proposals           common.Cache
	requestProposalChan chan *requestProposalInfo
	mining              ledger.Mining
	txnCollector        *transaction.TxnCollector

	proposalLock   sync.RWMutex
	proposalChan   chan *ledger.Block
	expectedHeight uint32

	nextConsensusHeightLock sync.Mutex
	nextConsensusHeight     uint32

	acceptedHeightLock sync.RWMutex
	acceptedHeight     uint32
}

// NewConsensus creates a MOCA consensus
func NewConsensus(account *vault.Account, localNode *node.LocalNode) (*Consensus, error) {
	txnCollector := transaction.NewTxnCollector(localNode.GetTxnPool(), maxNumTxnPerBlock)
	consensus := &Consensus{
		account:             account,
		localNode:           localNode,
		elections:           common.NewGoCache(cacheExpiration, cacheCleanupInterval),
		proposals:           common.NewGoCache(cacheExpiration, cacheCleanupInterval),
		proposalChan:        make(chan *ledger.Block, proposalChanLen),
		requestProposalChan: make(chan *requestProposalInfo, requestProposalChanLen),
		mining:              ledger.NewBuiltinMining(account, txnCollector),
		txnCollector:        txnCollector,
		expectedHeight:      ledger.DefaultLedger.Store.GetHeight() + 1,
	}
	return consensus, nil
}

// Start starts the consensus protocol
func (consensus *Consensus) Start() {
	consensus.startOnce.Do(func() {
		consensus.registerMessageHandler()
		go consensus.startConsensus()
		go consensus.startProposing()
		go consensus.startRequestingProposal()
		go consensus.startGettingNeighborConsensusState()
	})
}

// startConsensus starts the voting routine
func (consensus *Consensus) startConsensus() {
	for {
		consensus.maybeUpdateConsensusHeight()

		consensusHeight := consensus.GetExpectedHeight()

		if consensusHeight == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		elc, err := consensus.waitAndHandleProposal()
		if err != nil {
			log.Warningf("Handle proposal error: %v", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		consensus.setExpectedHeight(consensusHeight + 1)

		electedBlockHash, err := consensus.startElection(consensusHeight, elc)
		if err != nil {
			log.Errorf("Election error: %v", err)
			consensus.setExpectedHeight(consensusHeight)
			continue
		}

		if electedBlockHash == common.EmptyUint256 {
			log.Warningf("Reject block at height %d", consensusHeight)
			consensus.setExpectedHeight(consensusHeight)
			continue
		}

		log.Infof("Accept block %s at height %d", electedBlockHash.ToHexString(), consensusHeight)

		err = consensus.saveAcceptedBlock(electedBlockHash)
		if err != nil {
			log.Errorf("Error saving accepted block: %v", err)
			consensus.setExpectedHeight(consensusHeight)
			continue
		}

		consensus.setAcceptedHeight(consensusHeight)
	}
}

// startElection starts an election, sends out self vote, and returns election
// result after election stops.
func (consensus *Consensus) startElection(height uint32, elc *election.Election) (common.Uint256, error) {
	elc.Start()

	txVoteChan := elc.GetTxVoteChan()

	for vote := range txVoteChan {
		votedBlockHash, ok := vote.(common.Uint256)
		if !ok {
			log.Errorf("Convert vote %v to block hash error", vote)
		}

		err := consensus.vote(height, votedBlockHash)
		if err != nil {
			log.Errorf("Send vote error: %v", err)
		}
	}

	result, err := elc.GetResult()
	if err != nil {
		return common.EmptyUint256, err
	}

	electedBlockHash, ok := result.(common.Uint256)
	if !ok {
		return common.EmptyUint256, fmt.Errorf("Convert election result to block hash error")
	}

	return electedBlockHash, nil
}

// loadOrCreateElection loads or create an election with the given key. Returns
// the election, if the election is loaded, and error.
func (consensus *Consensus) loadOrCreateElection(key []byte) (*election.Election, bool, error) {
	if value, ok := consensus.elections.Get(key); ok && value != nil {
		if elc, ok := value.(*election.Election); ok && elc != nil {
			return elc, true, nil
		}
	}

	consensusNeighbors := consensus.localNode.GetNeighbors(func(rn *node.RemoteNode) bool {
		return rn.GetSyncState() == pb.PersistFinished
	})
	totalWeight := len(consensusNeighbors)
	if consensus.localNode.GetSyncState() == pb.PersistFinished {
		totalWeight++
	}

	config := &election.Config{
		Duration:                    electionDuration,
		MinVotingInterval:           minVotingInterval,
		ChangeVoteMinAbsoluteWeight: uint32(changeVoteMinRelativeWeight*float32(totalWeight) + 1),
		ConsensusMinAbsoluteWeight:  uint32(consensusMinRelativeWeight*float32(totalWeight) + 1),
	}

	elc, err := election.NewElection(config)
	if err != nil {
		return nil, false, err
	}

	err = consensus.elections.Set(key, elc)
	if err != nil {
		if value, ok := consensus.elections.Get(key); ok && value != nil {
			if elc, ok := value.(*election.Election); ok && elc != nil {
				return elc, true, nil
			}
		}
		return nil, false, err
	}

	return elc, false, nil
}

// GetExpectedHeight returns the expected consensus height
func (consensus *Consensus) GetExpectedHeight() uint32 {
	consensus.proposalLock.RLock()
	defer consensus.proposalLock.RUnlock()
	return consensus.expectedHeight
}

// setExpectedHeight sets the expected consensus height
func (consensus *Consensus) setExpectedHeight(expectedHeight uint32) {
	log.Infof("Change expected block height to %d", expectedHeight)

	consensus.proposalLock.Lock()
	if consensus.expectedHeight != expectedHeight {
		if expectedHeight < consensus.expectedHeight {
			for height := expectedHeight; height <= consensus.expectedHeight; height++ {
				consensus.elections.Set(heightToKey(height), nil)
			}
		}

		consensus.expectedHeight = expectedHeight
		consensus.proposalChan = make(chan *ledger.Block, proposalChanLen)
	}
	consensus.proposalLock.Unlock()
}

// setNextConsensusHeight sets the next consensus height that will be effective
// when current consensus finish.
func (consensus *Consensus) setNextConsensusHeight(height uint32) {
	consensus.nextConsensusHeightLock.Lock()
	consensus.nextConsensusHeight = height
	consensus.nextConsensusHeightLock.Unlock()
}

// GetAcceptedHeight gets the latest block height that has been accepted by
// consensus
func (consensus *Consensus) GetAcceptedHeight() uint32 {
	consensus.acceptedHeightLock.RLock()
	defer consensus.acceptedHeightLock.RUnlock()
	return consensus.acceptedHeight
}

// setAcceptedHeight sets the latest block height that has been accepted by
// consensus
func (consensus *Consensus) setAcceptedHeight(height uint32) {
	consensus.acceptedHeightLock.Lock()
	consensus.acceptedHeight = height
	consensus.acceptedHeightLock.Unlock()
}

// maybeUpdateConsensusHeight change expectedHeight to nextConsensusHeight if
// nextConsensusHeight is not zero.
func (consensus *Consensus) maybeUpdateConsensusHeight() {
	consensus.nextConsensusHeightLock.Lock()
	if consensus.nextConsensusHeight > 0 {
		consensus.setExpectedHeight(consensus.nextConsensusHeight)
		consensus.nextConsensusHeight = 0
	}
	consensus.nextConsensusHeightLock.Unlock()
}

func (consensus *Consensus) saveAcceptedBlock(electedBlockHash common.Uint256) error {
	block, err := consensus.getBlockProposal(electedBlockHash)
	if err != nil {
		return err
	}

	if block.Header.Height == ledger.DefaultLedger.Store.GetHeight()+1 {
		if consensus.localNode.GetSyncState() == pb.WaitForSyncing {
			consensus.localNode.SetSyncState(pb.PersistFinished)
		}
		return ledger.DefaultLedger.Blockchain.AddBlock(block)
	}

	if consensus.localNode.GetSyncState() == pb.PersistFinished {
		log.Infof("Accepted block height: %d, local ledger block height: %d, sync needed.", block.Header.Height, ledger.DefaultLedger.Store.GetHeight())
	}

	elc, loaded, err := consensus.loadOrCreateElection(heightToKey(block.Header.Height))
	if err != nil {
		return fmt.Errorf("Error load election: %v", err)
	}
	if !loaded {
		return fmt.Errorf("Election is created instead of loaded")
	}

	neighborIDs := elc.GetNeighborIDsByVote(electedBlockHash)
	neighbors := consensus.localNode.GetNeighbors(func(neighbor *node.RemoteNode) bool {
		for _, neighborID := range neighborIDs {
			if neighbor.GetID() == neighborID {
				return neighbor.GetHeight() > ledger.DefaultLedger.Store.GetHeight()
			}
		}
		return false
	})
	if len(neighbors) == 0 {
		return fmt.Errorf("Cannot get neighbors voted for block hash %s", electedBlockHash.ToHexString())
	}

	go func() {
		started, err := consensus.localNode.StartSyncing(block.Header.PrevBlockHash, block.Header.Height-1, neighbors)
		if err != nil {
			log.Errorf("Error syncing blocks: %v", err)
			if started {
				panic(err)
			}
		}
		if !started {
			return
		}

		defer consensus.localNode.ResetSyncing()

		err = consensus.saveBlocksAcceptedDuringSync(block.Header.Height)
		if err != nil {
			log.Errorf("Error saving blocks accepted during sync: %v", err)
			consensus.localNode.SetSyncState(pb.WaitForSyncing)
			return
		}

		consensus.localNode.SetSyncState(pb.PersistFinished)
	}()

	return nil
}

func (consensus *Consensus) saveBlocksAcceptedDuringSync(startHeight uint32) error {
	log.Infof("Start saving blocks accepted during sync")

	height := startHeight
	for height <= consensus.GetAcceptedHeight() {
		// FIXME: use sync save block api
		time.Sleep(300 * time.Millisecond)

		value, ok := consensus.elections.Get(heightToKey(height))
		if !ok || value == nil {
			return fmt.Errorf("Election at height %d not found in local cache", height)
		}

		elc, ok := value.(*election.Election)
		if !ok || elc == nil {
			return fmt.Errorf("Convert election at height %d from cache error", height)
		}

		result, err := elc.GetResult()
		if err != nil {
			return err
		}

		electedBlockHash, ok := result.(common.Uint256)
		if !ok {
			return fmt.Errorf("Convert election result to block hash error")
		}

		block, err := consensus.getBlockProposal(electedBlockHash)
		if err != nil {
			return err
		}

		err = ledger.DefaultLedger.Blockchain.AddBlock(block)
		if err != nil {
			return err
		}

		height++
	}

	log.Infof("Saved %d blocks accepted during sync", height-startHeight)

	return nil
}
