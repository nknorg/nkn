package moca

import (
	"bytes"
	"time"

	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/timer"
)

// startProposing starts the proposing routing
func (consensus *Consensus) startProposing() {
	var lastProposedHeight uint32
	proposingTimer := time.NewTimer(proposingStartDelay)
	for {
		select {
		case <-proposingTimer.C:
			currentHeight := ledger.DefaultLedger.Store.GetHeight()
			expectedHeight := consensus.GetExpectedHeight()
			timestamp := time.Now().Unix()
			if expectedHeight > lastProposedHeight && expectedHeight == currentHeight+1 && consensus.isBlockProposer(currentHeight, timestamp) {
				log.Infof("I am the block proposer at height %d", expectedHeight)

				block, err := consensus.proposeBlock(expectedHeight, timestamp)
				if err != nil {
					log.Errorf("Propose block %d at %v error: %v", expectedHeight, timestamp, err)
					break
				}

				blockHash := block.Header.Hash()
				log.Infof("Propose block %s at height %d", blockHash.ToHexString(), expectedHeight)

				err = consensus.receiveProposal(block)
				if err != nil {
					log.Error(err)
					break
				}

				lastProposedHeight = expectedHeight
				time.Sleep(electionStartDelay)
			}
		}
		timer.ResetTimer(proposingTimer, proposingInterval)
	}
}

// isBlockProposer returns if local node is the block proposer of block height+1
// at a given timestamp
func (consensus *Consensus) isBlockProposer(height uint32, timestamp int64) bool {
	nextPublicKey, nextChordID, _, err := ledger.GetNextBlockSigner(height, timestamp)
	if err != nil {
		log.Errorf("Get next block signer error: %v", err)
		return false
	}

	publickKey, err := consensus.account.PublicKey.EncodePoint(true)
	if err != nil {
		log.Errorf("Encode public key error: %v", err)
		return false
	}

	if !bytes.Equal(publickKey, nextPublicKey) {
		return false
	}

	if len(nextChordID) > 0 && !bytes.Equal(consensus.localNode.GetChordID(), nextChordID) {
		return false
	}

	return true
}

// proposeBlock proposes a new block at give height and timestamp
func (consensus *Consensus) proposeBlock(height uint32, timestamp int64) (*ledger.Block, error) {
	winnerHash, winnerType, err := ledger.GetNextMiningSigChainTxnHash(height)
	if err != nil {
		return nil, err
	}

	return consensus.mining.BuildBlock(height, consensus.localNode.GetChordID(), winnerHash, winnerType, timestamp)
}
