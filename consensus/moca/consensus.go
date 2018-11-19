package moca

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/moca/election"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/net/protocol"
	timer "github.com/nknorg/nkn/util/timer.go"
	"github.com/nknorg/nkn/vault"
	"github.com/nknorg/nnet/log"
)

const (
	electionStartDelay     = 10 * time.Second
	electionDuration       = 10 * time.Second
	minVotingInterval      = 500 * time.Millisecond
	proposingInterval      = 500 * time.Millisecond
	cacheExpiration        = 3600 * time.Second
	cacheCleanupInterval   = 600 * time.Second
	proposalChanLen        = 100
	requestProposalChanLen = 10000
)

// Consensus is the Majority vOte Cellular Automata (MOCA) consensus layer
type Consensus struct {
	account             *vault.Account
	localNode           protocol.Noder
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
}

type requestProposalInfo struct {
	neighborID uint64
	height     uint32
	blockHash  common.Uint256
}

// NewConsensus creates a MOCA consensus
func NewConsensus(account *vault.Account, localNode protocol.Noder) (*Consensus, error) {
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
	}
	return consensus, nil
}

// Start starts the consensus protocol
func (consensus *Consensus) Start() {
	consensus.startOnce.Do(func() {
		go consensus.startVoting()
		go consensus.startProposing()
	})
}

// startVoting starts the voting routine
func (consensus *Consensus) startVoting() {
	for {
		consensus.maybeUpdateConsensusHeight()

		consensusHeight := consensus.GetExpectedHeight()

		if consensusHeight == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		elc, err := consensus.handleProposal()
		if err != nil {
			log.Error(err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		consensus.setExpectedHeight(consensusHeight + 1)

		electedBlockHash, err := consensus.startElection(elc)
		if err != nil {
			log.Error(err)
			consensus.setExpectedHeight(consensusHeight)
			continue
		}

		if electedBlockHash == common.EmptyUint256 {
			log.Warningf("Reject block at height %d", consensusHeight)
			consensus.setExpectedHeight(consensusHeight)
			continue
		}

		// TODO: save block
	}
}

// requestProposal starts the request proposal routine
func (consensus *Consensus) requestProposal() {
	for {
		requestProposal := <-consensus.requestProposalChan

		expectedHeight := consensus.GetExpectedHeight()
		if requestProposal.height != expectedHeight {
			log.Warningf("Request invalid proposal height %d instead of %d", requestProposal.height, expectedHeight)
			continue
		}

		if requestProposal.blockHash == common.EmptyUint256 {
			log.Warning("Skip requesting empty block hash")
			continue
		}

		if _, ok := consensus.proposals.Get(requestProposal.blockHash.ToArray()); ok {
			continue
		}

		// TODO: request block proposal from neighbor
		block := &ledger.Block{}

		err := consensus.ReceiveProposal(block)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

// handleProposal waits for first valid proposal, and continues to handle
// proposal for electionStartDelay duration.
func (consensus *Consensus) handleProposal() (*election.Election, error) {
	var timerStartOnce sync.Once
	electionStartTimer := time.NewTimer(math.MaxInt64)
	electionStartTimer.Stop()
	timeoutTimer := time.NewTimer(electionStartDelay)
	validProposals := make(map[common.Uint256]*ledger.Block)

	consensus.proposalLock.RLock()
	consensusHeight := consensus.expectedHeight
	proposalChan := consensus.proposalChan
	consensus.proposalLock.RUnlock()

	elc, _, err := consensus.loadOrCreateElection(heightToKey(consensusHeight))
	if err != nil {
		return nil, err
	}

	if !ledger.CanVerifyHeight(consensusHeight) {
		for {
			if elc.NeighborVoteCount() > 0 {
				timerStartOnce.Do(func() {
					timer.StopTimer(timeoutTimer)
					electionStartTimer.Reset(electionStartDelay)
				})
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	for {
		select {
		case proposal := <-proposalChan:
			blockHash := proposal.Header.Hash()

			if !ledger.CanVerifyHeight(consensusHeight) {
				continue
			}

			err := ledger.SignerCheck(proposal.Header)
			if err != nil {
				log.Warningf("Ignore proposal that fails to pass signer check: %v", err)
				continue
			}

			timerStartOnce.Do(func() {
				timer.StopTimer(timeoutTimer)
				electionStartTimer.Reset(electionStartDelay)
			})

			acceptProposal := true

			err = ledger.HeaderCheck(proposal.Header)
			if err != nil {
				log.Warningf("Proposal fails to pass header check: %v", err)
				acceptProposal = false
			}

			err = ledger.TimestampCheck(proposal.Header.Timestamp)
			if err != nil {
				log.Warningf("Proposal fails to pass timestamp check: %v", err)
				acceptProposal = false
			}

			if acceptProposal {
				validProposals[blockHash] = proposal
				if len(validProposals) > 1 {
					log.Warningf("Received multiple different valid proposals")
					acceptProposal = false
				}
			}

			var initialVote common.Uint256
			if acceptProposal {
				initialVote = blockHash
			} else {
				// TODO: tell neighbor I have this block
			}

			elc.SetInitialVote(initialVote)

			// TODO: send out initial vote

		case <-electionStartTimer.C:
			return elc, nil

		case <-timeoutTimer.C:
			return nil, errors.New("Wait for proposal timeout")
		}
	}
}

// startElection starts an election, sends out self vote, and returns election
// result after election stops.
func (consensus *Consensus) startElection(elc *election.Election) (common.Uint256, error) {
	elc.Start()

	txVoteChan := elc.GetTxVoteChan()

	for vote := range txVoteChan {
		_ = vote
		// TODO: send vote to neighbor
	}

	result, err := elc.GetResult()
	if err != nil {
		return common.Uint256{}, err
	}

	electedBlockHash, ok := result.(common.Uint256)
	if !ok {
		return common.Uint256{}, errors.New("Invalid election result")
	}

	return electedBlockHash, nil
}

// loadOrCreateElection loads or create an election with the given key. Returns
// the election, if the election is loaded, and error.
func (consensus *Consensus) loadOrCreateElection(key []byte) (*election.Election, bool, error) {
	if value, ok := consensus.elections.Get(key); ok {
		if elc, ok := value.(*election.Election); ok {
			return elc, true, nil
		}
	}

	config := &election.Config{
		Duration:          electionDuration,
		MinVotingInterval: minVotingInterval,
	}

	elc, err := election.NewElection(config)
	if err != nil {
		return nil, false, err
	}

	err = consensus.elections.Add(key, elc)
	if err != nil {
		if value, ok := consensus.elections.Get(key); ok {
			if elc, ok := value.(*election.Election); ok {
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
	consensus.proposalLock.Lock()
	if consensus.expectedHeight != expectedHeight {
		consensus.expectedHeight = expectedHeight
		consensus.proposalChan = make(chan *ledger.Block, proposalChanLen)
	}
	consensus.proposalLock.Unlock()
}

// SetNextConsensusHeight sets the next consensus height that will be effective
// when current consensus finish.
func (consensus *Consensus) SetNextConsensusHeight(height uint32) {
	consensus.nextConsensusHeightLock.Lock()
	consensus.nextConsensusHeight = height
	consensus.nextConsensusHeightLock.Unlock()
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

// ReceiveProposal is called when a new proposal is received
func (consensus *Consensus) ReceiveProposal(block *ledger.Block) error {
	consensus.proposalLock.RLock()
	defer consensus.proposalLock.RUnlock()

	receivedHeight := block.Header.Height
	expectedHeight := consensus.expectedHeight
	if receivedHeight != expectedHeight {
		return fmt.Errorf("Receive invalid proposal height %d instead of %d", receivedHeight, expectedHeight)
	}

	select {
	case consensus.proposalChan <- block:
	default:
		return errors.New("Prososal chan full, discarding proposal")
	}

	blockHash := block.Header.Hash()
	consensus.proposals.Set(blockHash.ToArray(), block)

	return nil
}

// ReceiveVote is called when a vote from neighbor is received
func (consensus *Consensus) ReceiveVote(neighborID uint64, height uint32, blockHash common.Uint256) error {
	err := consensus.ReceiveBlockHash(neighborID, height, blockHash)
	if err != nil {
		log.Warningf("Receive block hash error when receive vote: %v", err)
	}

	elc, _, err := consensus.loadOrCreateElection(heightToKey(height))
	if err != nil {
		return err
	}

	err = elc.ReceiveVote(neighborID, blockHash)
	if err != nil {
		return err
	}

	return nil
}

// ReceiveBlockHash is called when a node receives a block hash from a neighbor
func (consensus *Consensus) ReceiveBlockHash(neighborID uint64, height uint32, blockHash common.Uint256) error {
	expectedHeight := consensus.GetExpectedHeight()
	if height != expectedHeight {
		return fmt.Errorf("Receive invalid block hash height %d instead of %d", height, expectedHeight)
	}

	if blockHash == common.EmptyUint256 {
		return errors.New("Receive empty block hash")
	}

	if _, ok := consensus.proposals.Get(blockHash.ToArray()); !ok {
		requestProposal := &requestProposalInfo{
			neighborID: neighborID,
			height:     height,
			blockHash:  blockHash,
		}

		select {
		case consensus.requestProposalChan <- requestProposal:
		default:
			return errors.New("Request prososal chan full")
		}
	}

	return nil
}
