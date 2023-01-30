package consensus

import (
	"bytes"
	"context"
	"time"

	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/util/timer"
)

// startProposing starts the proposing routing
func (consensus *Consensus) startProposing() {
	var currentHeight, expectedHeight, lastProposedHeight uint32
	var timestamp int64
	var ctx context.Context
	var cancel context.CancelFunc
	proposingTimer := time.NewTimer(proposingStartDelay)
	for {
		select {
		case <-proposingTimer.C:
			currentHeight = chain.DefaultLedger.Store.GetHeight()
			expectedHeight = consensus.GetExpectedHeight()
			timestamp = time.Now().Unix()
			if config.Parameters.Mining && expectedHeight > lastProposedHeight && expectedHeight == currentHeight+1 && consensus.isBlockProposer(currentHeight, timestamp) {
				log.Infof("I am the block proposer at height %d", expectedHeight)

				ctx, cancel = context.WithTimeout(context.Background(), proposingTimeout)

				b, err := consensus.proposeBlock(ctx, expectedHeight)
				if err != nil {
					log.Errorf("Propose block %d at %v error: %v", expectedHeight, timestamp, err)
					break
				}

				cancel()

				timestamp = time.Now().Unix()
				if !consensus.isBlockProposer(currentHeight, timestamp) {
					log.Errorf("I'm no longer the block proposer at height %d at %v", expectedHeight, timestamp)
					break
				}
				err = consensus.mining.SignBlock(b, timestamp)
				if err != nil {
					log.Errorf("Sign block with timestamp %v error: %v", timestamp, err)
					break
				}

				blockHash := b.Hash()
				log.Infof("Propose block %s at height %d", blockHash.ToHexString(), expectedHeight)

				consensus.localNode.IncrementProposalSubmitted()

				// Prevent neighbor from receiving proposal before last consensus stops
				time.Sleep(proposalPropagationDelay)

				err = consensus.receiveProposal(b)
				if err != nil {
					log.Error(err)
					break
				}

				lastProposedHeight = expectedHeight
			}
		}
		timer.ResetTimer(proposingTimer, proposingInterval)
	}
}

// isBlockProposer returns if local node is the block proposer of block height+1
// at a given timestamp
func (consensus *Consensus) isBlockProposer(height uint32, timestamp int64) bool {
	nextPublicKey, nextChordID, _, err := chain.GetNextBlockSigner(height, timestamp)
	if err != nil {
		log.Errorf("Get next block signer error: %v", err)
		return false
	}

	if !bytes.Equal(consensus.account.PublicKey, nextPublicKey) {
		return false
	}

	if len(nextChordID) > 0 && !bytes.Equal(consensus.localNode.GetChordID(), nextChordID) {
		return false
	}

	return true
}

// proposeBlock proposes a new block at give height and timestamp
func (consensus *Consensus) proposeBlock(ctx context.Context, height uint32) (*block.Block, error) {
	winnerHash, winnerType, err := chain.GetNextMiningSigChainTxnHash(height)
	if err != nil {
		return nil, err
	}

	return consensus.mining.BuildBlock(ctx, height, consensus.localNode.GetChordID(), winnerHash, winnerType)
}
