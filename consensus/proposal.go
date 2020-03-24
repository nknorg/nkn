package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/consensus/election"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/por"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/util/timer"
)

type requestProposalInfo struct {
	neighborID string
	height     uint32
	blockHash  common.Uint256
}

// getBlockProposal gets a proposal from proposal cache and convert to block
func (consensus *Consensus) getBlockProposal(blockHash common.Uint256) (*block.Block, error) {
	value, ok := consensus.proposals.Get(blockHash.ToArray())
	if !ok {
		return nil, fmt.Errorf("Block %s not found in local cache", blockHash.ToHexString())
	}

	block, ok := value.(*block.Block)
	if !ok {
		return nil, fmt.Errorf("Convert block %s from proposal cache error", blockHash.ToHexString())
	}

	return block, nil
}

func (consensus *Consensus) canVerifyHeight(height uint32) bool {
	return chain.CanVerifyHeight(height) && height >= consensus.localNode.GetMinVerifiableHeight()
}

// waitAndHandleProposal waits for first valid proposal, and continues to handle
// proposal for electionStartDelay duration.
func (consensus *Consensus) waitAndHandleProposal() (*election.Election, error) {
	var timerStartOnce sync.Once
	var verifyDeadline time.Time
	var initialVoteDeadline time.Time
	initialVote := common.EmptyUint256
	electionStartTimer := time.NewTimer(math.MaxInt64)
	electionStartTimer.Stop()
	timeoutTimer := time.NewTimer(electionStartDelay)
	proposals := make(map[common.Uint256]*block.Block)

	consensus.proposalLock.RLock()
	consensusHeight := consensus.expectedHeight
	proposalChan := consensus.proposalChan
	consensus.proposalLock.RUnlock()

	elc, _, err := consensus.loadOrCreateElection(consensusHeight)
	if err != nil {
		return nil, err
	}

	for {
		if consensus.canVerifyHeight(consensusHeight) {
			break
		}

		if elc.NeighborVoteCount() > 0 {
			timerStartOnce.Do(func() {
				timer.StopTimer(timeoutTimer)
				electionStartTimer.Reset(electionStartDelay)
				now := time.Now()
				verifyDeadline = now.Add(proposalVerificationTimeout)
				initialVoteDeadline = now.Add(initialVoteDelay)
			})
			break
		}

		select {
		case <-timeoutTimer.C:
			return nil, errors.New("Wait for neighbor vote timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	for {
		select {
		case proposal := <-proposalChan:
			blockHash := proposal.Hash()

			if !consensus.canVerifyHeight(consensusHeight) {
				err = consensus.iHaveBlockProposal(consensusHeight, blockHash)
				if err != nil {
					log.Errorf("Send I have block message error: %v", err)
				}
				continue
			}

			timerStartOnce.Do(func() {
				timer.StopTimer(timeoutTimer)
				electionStartTimer.Reset(electionStartDelay)
				now := time.Now()
				verifyDeadline = now.Add(proposalVerificationTimeout)
				initialVoteDeadline = now.Add(initialVoteDelay)
			})

			acceptProposal := true

			proposals[blockHash] = proposal
			if len(proposals) > 2 {
				log.Warningf("Received more than 2 different proposals, ignoring the rest")
				continue
			}

			err = consensus.iHaveBlockProposal(consensusHeight, blockHash)
			if err != nil {
				log.Errorf("Send I have block message error: %v", err)
			}

			if len(proposals) > 1 {
				log.Warningf("Received multiple different proposals, rejecting all of them")
				acceptProposal = false
			}

			verifyCtx, cancelVerify := context.WithDeadline(context.Background(), verifyDeadline)
			defer cancelVerify()

			if acceptProposal {
				if err = chain.TimestampCheck(proposal.Header, true); err != nil {
					log.Warningf("Proposal fails to pass soft timestamp check: %v", err)
					acceptProposal = false
				} else if err = chain.HeaderCheck(proposal); err != nil {
					log.Warningf("Proposal fails to pass header check: %v", err)
					acceptProposal = false
				} else if err = chain.NextBlockProposerCheck(proposal.Header); err != nil {
					log.Warningf("Proposal fails to pass next block proposal check: %v", err)
					acceptProposal = false
				} else if err = chain.TransactionCheck(verifyCtx, proposal); err != nil {
					log.Warningf("Proposal fails to pass transaction check: %v", err)
					acceptProposal = false
				}
			}

			if acceptProposal {
				initialVote = blockHash
			}

			elc.SetInitialVote(initialVote)

			initialVoteCtx, cancelInitialVote := context.WithDeadline(context.Background(), initialVoteDeadline)
			defer cancelInitialVote()

			go func(ctx context.Context, vote common.Uint256) {
				<-ctx.Done()
				if vote != initialVote {
					return
				}
				err := consensus.vote(consensusHeight, vote)
				if err != nil {
					log.Errorf("Send initial vote error: %v", err)
				}
			}(initialVoteCtx, initialVote)

			select {
			case <-electionStartTimer.C:
				return elc, nil
			default:
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

		block, err := consensus.requestProposal(neighbor, requestProposal.blockHash, requestProposal.height, requestTransactionType)
		if err != nil {
			log.Errorf("Request block %s error: %v", requestProposal.blockHash.ToHexString(), err)
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
func (consensus *Consensus) receiveProposal(block *block.Block) error {
	blockHash := block.Hash()

	log.Infof("Receive block proposal %s (%d txn, %d bytes) by %x", blockHash.ToHexString(), len(block.Transactions), block.GetTxsSize(), block.Header.UnsignedHeader.SignerPk)

	consensus.proposalLock.RLock()
	defer consensus.proposalLock.RUnlock()

	receivedHeight := block.Header.UnsignedHeader.Height
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
	log.Debugf("Receive block hash %s for height %d from neighbor %v", blockHash.ToHexString(), height, neighborID)

	if blockHash == common.EmptyUint256 {
		return errors.New("Receive empty block hash")
	}

	if _, ok := consensus.proposals.Get(blockHash.ToArray()); ok {
		return nil
	}

	if _, ok := consensus.neighborBlacklist.Load(neighborID); ok {
		return fmt.Errorf("ignore block hash %s from blacklist neighbor %s", blockHash.ToHexString(), neighborID)
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
func (consensus *Consensus) requestProposal(neighbor *node.RemoteNode, blockHash common.Uint256, height uint32, requestType pb.RequestTransactionType) (*block.Block, error) {
	var shortHashSalt []byte
	var shortHashSize uint32
	if requestType == pb.REQUEST_TRANSACTION_SHORT_HASH {
		shortHashSalt = config.ShortHashSalt
		shortHashSize = config.ShortHashSize
	}

	msg, err := NewRequestBlockProposalMessage(blockHash, requestType, shortHashSalt, shortHashSize)
	if err != nil {
		return nil, err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, false)
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

	b := &block.Block{}
	b.FromMsgBlock(replyMsg.Block)

	if b.Header.UnsignedHeader.Height != height {
		return nil, fmt.Errorf("Received block height %d is different from expected height %d", b.Header.UnsignedHeader.Height, height)
	}

	receivedBlockHash := b.Hash()
	if receivedBlockHash != blockHash {
		return nil, fmt.Errorf("Received block hash %s is different from requested hash %s", receivedBlockHash.ToHexString(), blockHash.ToHexString())
	}

	if consensus.canVerifyHeight(b.Header.UnsignedHeader.Height) {
		// We put hard timestamp check here to prevent proposal with invalid
		// timestamp to be propagated
		if err = chain.TimestampCheck(b.Header, false); err != nil {
			consensus.proposals.Set(blockHash.ToArray(), b)
			return nil, fmt.Errorf("Proposal fails to pass hard timestamp check: %v", err)
		}
		if err = chain.SignerCheck(b.Header); err != nil {
			consensus.proposals.Set(blockHash.ToArray(), b)
			return nil, fmt.Errorf("Proposal fails to pass signer check: %v", err)
		}
	}

	if err = chain.SignatureCheck(b.Header); err != nil {
		err = fmt.Errorf("Proposal fails to pass signature check: %v", err)
		consensus.neighborBlacklist.Store(neighbor.GetID(), err)
		log.Infof("Add neighbor %s to blacklist because: %v", neighbor.GetID(), err)
		return nil, err
	}

	var txnsRoot common.Uint256
	poolTxns := make([]*transaction.Transaction, 0, len(replyMsg.TransactionsHash))
	missingTxnsHash := make([][]byte, 0, len(replyMsg.TransactionsHash))

	switch requestType {
	case pb.REQUEST_FULL_TRANSACTION:
		txnsHash := make([]common.Uint256, len(b.Transactions))
		for i, txn := range b.Transactions {
			txnsHash[i] = txn.Hash()
		}

		if hasDuplicateHash(txnsHash) {
			return nil, fmt.Errorf("Block txn list contains duplicate")
		}

		txnsRoot, err = crypto.ComputeRoot(txnsHash)
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(txnsRoot.ToArray(), b.Header.UnsignedHeader.TransactionsRoot) {
			return nil, fmt.Errorf("Computed txn root %x is different from txn root in header %x", txnsRoot.ToArray(), b.Header.UnsignedHeader.TransactionsRoot)
		}

		return b, nil
	case pb.REQUEST_TRANSACTION_HASH:
		txnsHash := make([]common.Uint256, len(replyMsg.TransactionsHash))
		for i, txnHashBytes := range replyMsg.TransactionsHash {
			txnsHash[i], err = common.Uint256ParseFromBytes(txnHashBytes)
			if err != nil {
				return nil, err
			}
		}

		if hasDuplicateHash(txnsHash) {
			return nil, fmt.Errorf("Txn hash list contains duplicate")
		}

		txnsRoot, err = crypto.ComputeRoot(txnsHash)
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(txnsRoot.ToArray(), b.Header.UnsignedHeader.TransactionsRoot) {
			return nil, fmt.Errorf("Computed txn root %x is different from txn root in header %x", txnsRoot.ToArray(), b.Header.UnsignedHeader.TransactionsRoot)
		}

		for i := range txnsHash {
			if txn := consensus.localNode.TxnPool.GetTxnByHash(txnsHash[i]); txn != nil {
				poolTxns = append(poolTxns, txn)
			} else if txn, err = por.GetPorServer().GetSigChainTxn(txnsHash[i]); err == nil && txn != nil {
				poolTxns = append(poolTxns, txn)
			} else {
				missingTxnsHash = append(missingTxnsHash, txnsHash[i].ToArray())
			}
		}
	case pb.REQUEST_TRANSACTION_SHORT_HASH:
		for i := range replyMsg.TransactionsHash {
			if txn := consensus.localNode.TxnPool.GetTxnByShortHash(replyMsg.TransactionsHash[i]); txn != nil {
				poolTxns = append(poolTxns, txn)
			} else if txn, err = por.GetPorServer().GetSigChainTxnByShortHash(replyMsg.TransactionsHash[i]); err == nil && txn != nil {
				poolTxns = append(poolTxns, txn)
			} else {
				missingTxnsHash = append(missingTxnsHash, replyMsg.TransactionsHash[i])
			}
		}
	default:
		return nil, fmt.Errorf("Unsupported request type %v", requestType)
	}

	log.Infof("Receive block info %s, %d txn found in pool, %d txn to request", blockHash.ToHexString(), len(poolTxns), len(missingTxnsHash))

	requestedTxns, err := consensus.requestProposalTransactions(neighbor, blockHash, requestType, missingTxnsHash)
	if err != nil {
		return nil, err
	}

	mergedTxns := make([]*transaction.Transaction, len(replyMsg.TransactionsHash))

	err = mergeTxns(poolTxns, requestedTxns, mergedTxns, replyMsg.TransactionsHash, requestType)
	if err != nil {
		return nil, err
	}

	if requestType == pb.REQUEST_TRANSACTION_SHORT_HASH {
		txnsHash := make([]common.Uint256, len(replyMsg.TransactionsHash))
		for i, txn := range mergedTxns {
			txnsHash[i] = txn.Hash()
		}

		if hasDuplicateHash(txnsHash) {
			return nil, fmt.Errorf("Txn list contains duplicate")
		}

		txnsRoot, err = crypto.ComputeRoot(txnsHash)
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(txnsRoot.ToArray(), b.Header.UnsignedHeader.TransactionsRoot) {
			log.Warningf("Computed txn root %x is different from txn root in header %x, fall back to request full txn hash.", txnsRoot.ToArray(), b.Header.UnsignedHeader.TransactionsRoot)
			return consensus.requestProposal(neighbor, blockHash, height, pb.REQUEST_TRANSACTION_HASH)
		}
	}

	b.Transactions = mergedTxns

	return b, nil
}

func getTxnHash(txn *transaction.Transaction, requestType pb.RequestTransactionType) ([]byte, error) {
	var txnHashBytes []byte
	switch requestType {
	case pb.REQUEST_TRANSACTION_HASH:
		txnHash := txn.Hash()
		txnHashBytes = txnHash.ToArray()
	case pb.REQUEST_TRANSACTION_SHORT_HASH:
		txnHashBytes = txn.ShortHash(config.ShortHashSalt, config.ShortHashSize)
	default:
		return nil, fmt.Errorf("Unsupported request type %v", requestType)
	}
	return txnHashBytes, nil
}

func mergeTxns(poolTxns, requestedTxns, mergedTxns []*transaction.Transaction, txnsHash [][]byte, requestType pb.RequestTransactionType) error {
	if len(mergedTxns) != len(txnsHash) {
		return fmt.Errorf("Merged txn array len %d is different from txn hash array len %d", len(mergedTxns), len(txnsHash))
	}
	if len(poolTxns)+len(requestedTxns) != len(txnsHash) {
		return fmt.Errorf("Sum of pool txn array len %d and requested txn array len %d is different from txn hash array len %d", len(poolTxns), len(requestedTxns), len(txnsHash))
	}

	i, j := 0, 0
	for {
		if i+j == len(mergedTxns) {
			break
		}

		if i < len(poolTxns) {
			txnHashBytes, err := getTxnHash(poolTxns[i], requestType)
			if err != nil {
				return err
			}
			if bytes.Equal(txnHashBytes, txnsHash[i+j]) {
				mergedTxns[i+j] = poolTxns[i]
				i++
				continue
			}
		}

		if j < len(requestedTxns) {
			txnHashBytes, err := getTxnHash(requestedTxns[j], requestType)
			if err != nil {
				return err
			}
			if bytes.Equal(txnHashBytes, txnsHash[i+j]) {
				mergedTxns[i+j] = requestedTxns[j]
				j++
				continue
			}
		}

		return errors.New("Merge pool and requested txn array error: txn hash mismatch")
	}

	return nil
}

func (consensus *Consensus) requestProposalTransactions(neighbor *node.RemoteNode, blockHash common.Uint256, requestType pb.RequestTransactionType, txnsHash [][]byte) ([]*transaction.Transaction, error) {
	var shortHashSalt []byte
	var shortHashSize uint32
	if requestType == pb.REQUEST_TRANSACTION_SHORT_HASH {
		shortHashSalt = config.ShortHashSalt
		shortHashSize = config.ShortHashSize
	}

	msg, err := NewRequestProposalTransactionsMessage(blockHash, requestType, shortHashSalt, shortHashSize, txnsHash)
	if err != nil {
		return nil, err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, false)
	if err != nil {
		return nil, err
	}

	replyBytes, err := neighbor.SendBytesSync(buf)
	if err != nil {
		return nil, err
	}

	replyMsg := &pb.RequestProposalTransactionsReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}

	if len(replyMsg.Transactions) != len(txnsHash) {
		return nil, fmt.Errorf("Returned txn count %d is different from requested count %d", len(replyMsg.Transactions), len(txnsHash))
	}

	txns := make([]*transaction.Transaction, len(replyMsg.Transactions))
	for i, txn := range replyMsg.Transactions {
		txns[i] = &transaction.Transaction{Transaction: txn}
	}

	for i := range txns {
		txnHash, err := getTxnHash(txns[i], requestType)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(txnHash, txnsHash[i]) {
			return nil, fmt.Errorf("The %dth txn hash %x is different from expected %x", i, txnHash, txnsHash[i])
		}
	}

	return txns, nil
}

// iHaveBlockProposal sends I_HAVE_BLOCK_PROPOSAL message to neighbors informing
// them node has a block proposal
func (consensus *Consensus) iHaveBlockProposal(height uint32, blockHash common.Uint256) error {
	msg, err := NewIHaveBlockProposalMessage(height, blockHash)
	if err != nil {
		return err
	}

	buf, err := consensus.localNode.SerializeMessage(msg, false)
	if err != nil {
		return err
	}

	for _, neighbor := range consensus.localNode.GetGossipNeighbors(nil) {
		err = neighbor.SendBytesAsync(buf)
		if err != nil {
			log.Errorf("Send vote to neighbor %v error: %v", neighbor, err)
			continue
		}
	}

	return nil
}
