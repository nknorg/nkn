package moca

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/moca/election"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/timer"
)

type requestProposalInfo struct {
	neighborID string
	height     uint32
	blockHash  common.Uint256
}

// getBlockProposal gets a proposal from proposal cache and convert to block
func (consensus *Consensus) getBlockProposal(blockHash common.Uint256) (*ledger.Block, error) {
	value, ok := consensus.proposals.Get(blockHash.ToArray())
	if !ok {
		return nil, fmt.Errorf("Block %s not found in local cache", blockHash.ToHexString())
	}

	block, ok := value.(*ledger.Block)
	if !ok {
		return nil, fmt.Errorf("Convert block %s from proposal cache error", blockHash.ToHexString())
	}

	return block, nil
}

// waitAndHandleProposal waits for first valid proposal, and continues to handle
// proposal for electionStartDelay duration.
func (consensus *Consensus) waitAndHandleProposal() (*election.Election, error) {
	var timerStartOnce sync.Once
	electionStartTimer := time.NewTimer(math.MaxInt64)
	electionStartTimer.Stop()
	timeoutTimer := time.NewTimer(electionStartDelay)
	proposals := make(map[common.Uint256]*ledger.Block)

	consensus.proposalLock.RLock()
	consensusHeight := consensus.expectedHeight
	proposalChan := consensus.proposalChan
	consensus.proposalLock.RUnlock()

	elc, _, err := consensus.loadOrCreateElection(heightToKey(consensusHeight))
	if err != nil {
		return nil, err
	}

	for {
		if ledger.CanVerifyHeight(consensusHeight) {
			break
		}
		if elc.NeighborVoteCount() > 0 {
			timerStartOnce.Do(func() {
				timer.StopTimer(timeoutTimer)
				electionStartTimer.Reset(electionStartDelay)
			})
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	for {
		select {
		case proposal := <-proposalChan:
			blockHash := proposal.Header.Hash()

			if !ledger.CanVerifyHeight(consensusHeight) {
				err = consensus.iHaveProposal(consensusHeight, blockHash)
				if err != nil {
					log.Errorf("Send I have block message error: %v", err)
				}
				continue
			}

			err = ledger.TimestampCheck(proposal.Header.Timestamp)
			if err != nil {
				log.Warningf("Ignore proposal that fails to pass timestamp check: %v", err)
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

			proposals[blockHash] = proposal
			if len(proposals) > 2 {
				log.Warningf("Received more than 2 different proposals, ignoring the rest to prevent spam")
				continue
			}
			if len(proposals) > 1 {
				log.Warningf("Received multiple different proposals, rejecting all of them")
				acceptProposal = false
			}

			err = ledger.HeaderCheck(proposal.Header)
			if err != nil {
				log.Warningf("Proposal fails to pass header check: %v", err)
				acceptProposal = false
			}

			err = ledger.NextBlockProposerCheck(proposal)
			if err != nil {
				log.Warningf("Proposal fails to pass next block proposal check: %v", err)
				acceptProposal = false
			}

			err = ledger.TransactionCheck(proposal)
			if err != nil {
				log.Warningf("Proposal fails to pass transaction check: %v", err)
				acceptProposal = false
			}

			initialVote := common.EmptyUint256
			if acceptProposal {
				initialVote = blockHash
			} else {
				err = consensus.iHaveProposal(consensusHeight, blockHash)
				if err != nil {
					log.Errorf("Send I have block message error: %v", err)
				}
			}

			elc.SetInitialVote(initialVote)

			err = consensus.vote(consensusHeight, initialVote)
			if err != nil {
				log.Errorf("Send initial vote error: %v", err)
			}

		case <-electionStartTimer.C:
			return elc, nil

		case <-timeoutTimer.C:
			return nil, errors.New("Wait for proposal timeout")
		}
	}
}

// startRequestProposal starts the request proposal routine
func (consensus *Consensus) startRequestingProposal() {
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

		neighbor := consensus.localNode.GetNbrNode(requestProposal.neighborID)
		if neighbor == nil {
			continue
		}

		log.Infof("Request block %s from neighbor %v", requestProposal.blockHash.ToHexString(), neighbor.GetID())

		block, err := consensus.requestProposal(neighbor, requestProposal.blockHash)
		if err != nil {
			log.Errorf("Request block error: %v", err)
			continue
		}
		if block == nil {
			log.Warning("Request block msg returned empty block from neighbor %v", neighbor.GetID())
			continue
		}

		err = consensus.receiveProposal(block)
		if err != nil {
			log.Warningf("Receive proposal error: %v", err)
			continue
		}
	}
}

// receiveProposal is called when a new proposal is received
func (consensus *Consensus) receiveProposal(block *ledger.Block) error {
	blockHash := block.Header.Hash()

	log.Debugf("Receive block proposal %s", blockHash.ToHexString())

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

	consensus.proposals.Set(blockHash.ToArray(), block)

	return nil
}

// receiveProposalHash is called when a node receives a block proposal hash from
// a neighbor
func (consensus *Consensus) receiveProposalHash(neighborID string, height uint32, blockHash common.Uint256) error {
	log.Debugf("Receive block hash %s for height %d from neighbor %d", blockHash.ToHexString(), height, neighborID)

	if blockHash == common.EmptyUint256 {
		return errors.New("Receive empty block hash")
	}

	if _, ok := consensus.proposals.Get(blockHash.ToArray()); ok {
		return nil
	}

	expectedHeight := consensus.GetExpectedHeight()
	if height != expectedHeight {
		return fmt.Errorf("Receive invalid block hash height %d instead of %d", height, expectedHeight)
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

// requestProposal requests a block proposal by block hash from a neighbor using
// REQUEST_BLOCK_PROPOSAL message
func (consensus *Consensus) requestProposal(neighbor *node.RemoteNode, blockHash common.Uint256) (*ledger.Block, error) {
	msg, err := NewRequestBlockProposalMessage(blockHash)
	if err != nil {
		return nil, err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, true)
	if err != nil {
		return nil, err
	}

	replyBytes, err := neighbor.SendBytesSync(buf)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.RequestBlockProposalReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	if len(replyMsg.Block) == 0 {
		return nil, nil
	}

	block := &ledger.Block{}
	err = block.Deserialize(bytes.NewReader(replyMsg.Block))
	if err != nil {
		return nil, err
	}

	return block, nil
}

// iHaveProposal sends I_HAVE_PROPOSAL message to neighbors informing them node
// has a block proposal
func (consensus *Consensus) iHaveProposal(height uint32, blockHash common.Uint256) error {
	msg, err := NewIHaveBlockProposalMessage(height, blockHash)
	if err != nil {
		return err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, true)
	if err != nil {
		return err
	}

	for _, neighbor := range consensus.localNode.GetNeighbors(nil) {
		err = neighbor.SendBytesAsync(buf)
		if err != nil {
			log.Errorf("Send vote to neighbor %v error: %v", neighbor, err)
		}
	}

	return nil
}
