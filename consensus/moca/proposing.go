package moca

import (
	"bytes"
	"time"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/por"
	timer "github.com/nknorg/nkn/util/timer.go"
	"github.com/nknorg/nnet/log"
)

const (
	maxNumTxnPerBlock = 20480
)

// startProposing starts the proposing routing
func (consensus *Consensus) startProposing() {
	proposingTimer := time.NewTimer(proposingInterval)
	for {
		select {
		case <-proposingTimer.C:
			currentHeight := ledger.DefaultLedger.Store.GetHeight()
			expectedHeight := consensus.GetExpectedHeight()
			timestamp := time.Now().Unix()
			if expectedHeight == currentHeight+1 && consensus.isBlockProposer(currentHeight, timestamp) {
				block, err := consensus.proposeBlock(expectedHeight, timestamp)
				if err != nil {
					log.Error(err)
					break
				}

				err = consensus.ReceiveProposal(block)
				if err != nil {
					log.Error(err)
					break
				}

				time.Sleep(electionStartDelay)
			}
		}
		timer.ResetTimer(proposingTimer, proposingInterval)
	}
}

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

	if !bytes.Equal(consensus.localNode.GetChordAddr(), nextChordID) {
		return false
	}

	return true
}

func (consensus *Consensus) proposeBlock(height uint32, timestamp int64) (*ledger.Block, error) {
	var winnerHash common.Uint256
	var winnerType ledger.WinnerType

	if height < ledger.NumGenesisBlocks {
		winnerHash = common.EmptyUint256
		winnerType = ledger.GenesisSigner
	} else {
		nextMiningSigChainTxnHash, err := por.GetPorServer().GetMiningSigChainTxnHash(height + 1)
		if err != nil {
			return nil, err
		}

		nextMiningTxn, err := consensus.getTxnByHash(nextMiningSigChainTxnHash)
		if err != nil {
			return nil, err
		}

		winnerHash = nextMiningTxn.Hash()
		winnerType = ledger.TxnSigner
	}

	return consensus.mining.BuildBlock(height, consensus.localNode.GetChordAddr(), winnerHash, winnerType, timestamp)
}

func (consensus *Consensus) getTxnByHash(txnHash common.Uint256) (*transaction.Transaction, error) {
	txnInPool := consensus.txnCollector.GetTransaction(txnHash)
	if txnInPool != nil {
		return txnInPool, nil
	}

	return ledger.DefaultLedger.Store.GetTransaction(txnHash)
}
