package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/consensus/election"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/por"
	"github.com/nknorg/nkn/v2/transaction"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/util/timer"
)

type requestProposalInfo struct {
	neighborID string
	height     uint32
	blockHash  common.Uint256
}

type proposalInfo struct {
	block        *block.Block
	receivedTime time.Time
}

// getBlockProposal gets a proposal from proposal cache and convert to block
func (consensus *Consensus) getBlockProposal(blockHash common.Uint256) (*block.Block, error) {
	value, ok := consensus.proposals.Get(blockHash.ToArray())
	if !ok {
		return nil, fmt.Errorf("block %s not found in local cache", blockHash.ToHexString())
	}

	block, ok := value.(*block.Block)
	if !ok {
		return nil, fmt.Errorf("convert block %s from proposal cache error", blockHash.ToHexString())
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
				electionStartTimer.Reset(electionStartDelay - initialVoteDelay)
				now := time.Now()
				verifyDeadline = now.Add(proposalVerificationTimeout - initialVoteDelay)
				initialVoteDeadline = now
			})
			break
		}

		select {
		case <-timeoutTimer.C:
			return nil, errors.New("wait for neighbor vote timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	proposalCount := 0
	for {
		select {
		case proposalInfo := <-proposalChan:
			proposalCount++
			proposal := proposalInfo.block
			receivedTime := proposalInfo.receivedTime
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
				electionStartTimer.Reset(electionStartDelay - time.Since(receivedTime))
				verifyDeadline = receivedTime.Add(proposalVerificationTimeout)
				initialVoteDeadline = receivedTime.Add(initialVoteDelay)
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
				} else if err = chain.HeaderCheck(proposal.Header, false); err != nil {
					log.Warningf("Proposal fails to pass header check: %v", err)
					acceptProposal = false
				} else if err = chain.NextBlockProposerCheck(proposal.Header); err != nil {
					log.Warningf("Proposal fails to pass next block proposal check: %v", err)
					acceptProposal = false
				} else if err = chain.TransactionCheck(verifyCtx, proposal, false); err != nil {
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
			if proposalCount > 0 {
				return elc, nil
			}
			return nil, errors.New("election wait for proposal timeout")

		case <-timeoutTimer.C:
			return nil, errors.New("wait for proposal timeout")
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

		neighbor := consensus.localNode.GetNeighborNode(requestProposal.neighborID)
		if neighbor == nil {
			continue
		}

		log.Infof("Request block %s from neighbor %v", requestProposal.blockHash.ToHexString(), neighbor.GetID())

		block, err := consensus.requestProposal(neighbor, requestProposal.blockHash, requestProposal.height, defaultRequestTransactionType)
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
		return fmt.Errorf("receive invalid proposal height %d instead of %d", receivedHeight, expectedHeight)
	}

	select {
	case consensus.proposalChan <- &proposalInfo{block: block, receivedTime: time.Now()}:
	default:
		return errors.New("prososal chan full, discarding proposal")
	}

	consensus.proposals.Set(blockHash.ToArray(), block)

	return nil
}

// receiveProposalHash is called when a node receives a block proposal hash from
// a neighbor
func (consensus *Consensus) receiveProposalHash(neighborID string, height uint32, blockHash common.Uint256) error {
	log.Debugf("Receive block hash %s for height %d from neighbor %v", blockHash.ToHexString(), height, neighborID)

	if blockHash == common.EmptyUint256 {
		return errors.New("receive empty block hash")
	}

	if _, ok := consensus.proposals.Get(blockHash.ToArray()); ok {
		return nil
	}

	if _, ok := consensus.neighborBlocklist.Load(neighborID); ok {
		return fmt.Errorf("ignore block hash %s from blocklist neighbor %s", blockHash.ToHexString(), neighborID)
	}

	expectedHeight := consensus.GetExpectedHeight()
	if height != expectedHeight {
		return fmt.Errorf("receive invalid block hash height %d instead of %d", height, expectedHeight)
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
			return errors.New("request prososal chan full")
		}
	}

	return nil
}

// requestProposal requests a block proposal by block hash from a neighbor using
// REQUEST_BLOCK_PROPOSAL message
func (consensus *Consensus) requestProposal(neighbor *node.RemoteNode, blockHash common.Uint256, height uint32, requestType pb.RequestTransactionType) (*block.Block, error) {
	var shortHashSalt []byte
	var shortHashSize uint32
	if requestType == pb.RequestTransactionType_REQUEST_TRANSACTION_SHORT_HASH {
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
		return nil, fmt.Errorf("received block height %d is different from expected height %d", b.Header.UnsignedHeader.Height, height)
	}

	receivedBlockHash := b.Hash()
	if receivedBlockHash != blockHash {
		return nil, fmt.Errorf("received block hash %s is different from requested hash %s", receivedBlockHash.ToHexString(), blockHash.ToHexString())
	}

	if consensus.canVerifyHeight(b.Header.UnsignedHeader.Height) {
		// We put hard timestamp check here to prevent proposal with invalid
		// timestamp to be propagated
		if err = chain.TimestampCheck(b.Header, false); err != nil {
			consensus.proposals.Set(blockHash.ToArray(), b)
			return nil, fmt.Errorf("proposal fails to pass hard timestamp check: %v", err)
		}
		if err = chain.SignerCheck(b.Header); err != nil {
			consensus.proposals.Set(blockHash.ToArray(), b)
			return nil, fmt.Errorf("proposal fails to pass signer check: %v", err)
		}
	}

	if err = b.Header.VerifySignature(); err != nil {
		err = fmt.Errorf("proposal fails to pass signature check: %v", err)
		consensus.neighborBlocklist.Store(neighbor.GetID(), err)
		log.Infof("Add neighbor %s to blocklist because: %v", neighbor.GetID(), err)
		return nil, err
	}

	existingTxns := make([]*transaction.Transaction, 0, len(replyMsg.TransactionsHash))
	missingTxnsHash := make([][]byte, 0)

	switch requestType {
	case pb.RequestTransactionType_REQUEST_FULL_TRANSACTION:
		txnsHash := make([]common.Uint256, len(b.Transactions))
		for i, txn := range b.Transactions {
			txnsHash[i] = txn.Hash()
		}

		err = crypto.VerifyRoot(txnsHash, b.Header.UnsignedHeader.TransactionsRoot)
		if err != nil {
			return nil, err
		}

		return b, nil
	case pb.RequestTransactionType_REQUEST_TRANSACTION_HASH:
		txnsHash := make([]common.Uint256, len(replyMsg.TransactionsHash))
		for i, txnHashBytes := range replyMsg.TransactionsHash {
			txnsHash[i], err = common.Uint256ParseFromBytes(txnHashBytes)
			if err != nil {
				return nil, err
			}
		}

		err = crypto.VerifyRoot(txnsHash, b.Header.UnsignedHeader.TransactionsRoot)
		if err != nil {
			return nil, err
		}

		blockTxns := make(map[common.Uint256]*transaction.Transaction, len(b.Transactions))
		for _, txn := range b.Transactions {
			blockTxns[txn.Hash()] = txn
		}

		for i := range txnsHash {
			if txn := blockTxns[txnsHash[i]]; txn != nil {
				existingTxns = append(existingTxns, txn)
			} else if txn = consensus.localNode.TxnPool.GetTxnByHash(txnsHash[i]); txn != nil {
				existingTxns = append(existingTxns, txn)
			} else if txn, err = por.GetPorServer().GetSigChainTxn(txnsHash[i]); err == nil && txn != nil {
				existingTxns = append(existingTxns, txn)
			} else {
				missingTxnsHash = append(missingTxnsHash, txnsHash[i].ToArray())
			}
		}
	case pb.RequestTransactionType_REQUEST_TRANSACTION_SHORT_HASH:
		blockTxns := make(map[string]*transaction.Transaction, len(b.Transactions))
		for _, txn := range b.Transactions {
			blockTxns[string(txn.ShortHash(config.ShortHashSalt, config.ShortHashSize))] = txn
		}

		for i := range replyMsg.TransactionsHash {
			if txn := blockTxns[string(replyMsg.TransactionsHash[i])]; txn != nil {
				existingTxns = append(existingTxns, txn)
			} else if txn := consensus.localNode.TxnPool.GetTxnByShortHash(replyMsg.TransactionsHash[i]); txn != nil {
				existingTxns = append(existingTxns, txn)
			} else if txn, err = por.GetPorServer().GetSigChainTxnByShortHash(replyMsg.TransactionsHash[i]); err == nil && txn != nil {
				existingTxns = append(existingTxns, txn)
			} else {
				missingTxnsHash = append(missingTxnsHash, replyMsg.TransactionsHash[i])
			}
		}
	default:
		return nil, fmt.Errorf("unsupported request type %v", requestType)
	}

	log.Infof("Receive block info %s, %d txn found, %d txn to request", blockHash.ToHexString(), len(existingTxns), len(missingTxnsHash))

	var mergedTxns []*transaction.Transaction
	if len(missingTxnsHash) > 0 {
		requestedTxns, err := consensus.requestProposalTransactions(neighbor, blockHash, requestType, missingTxnsHash)
		if err != nil {
			return nil, err
		}

		mergedTxns = make([]*transaction.Transaction, len(replyMsg.TransactionsHash))

		err = mergeTxns(existingTxns, requestedTxns, mergedTxns, replyMsg.TransactionsHash, requestType)
		if err != nil {
			return nil, err
		}
	} else {
		mergedTxns = existingTxns
	}

	if requestType == pb.RequestTransactionType_REQUEST_TRANSACTION_SHORT_HASH {
		txnsHash := make([]common.Uint256, len(mergedTxns))
		for i, txn := range mergedTxns {
			txnsHash[i] = txn.Hash()
		}

		err = crypto.VerifyRoot(txnsHash, b.Header.UnsignedHeader.TransactionsRoot)
		if err != nil {
			log.Warningf("Short hash verify root error: %v, fallback to full transaction hash", err)
			return consensus.requestProposal(neighbor, blockHash, height, pb.RequestTransactionType_REQUEST_TRANSACTION_HASH)
		}
	}

	b.Transactions = mergedTxns

	return b, nil
}

func getTxnHash(txn *transaction.Transaction, requestType pb.RequestTransactionType) ([]byte, error) {
	var txnHashBytes []byte
	switch requestType {
	case pb.RequestTransactionType_REQUEST_TRANSACTION_HASH:
		txnHash := txn.Hash()
		txnHashBytes = txnHash.ToArray()
	case pb.RequestTransactionType_REQUEST_TRANSACTION_SHORT_HASH:
		txnHashBytes = txn.ShortHash(config.ShortHashSalt, config.ShortHashSize)
	default:
		return nil, fmt.Errorf("unsupported request type %v", requestType)
	}
	return txnHashBytes, nil
}

func mergeTxns(existingTxns, requestedTxns, mergedTxns []*transaction.Transaction, txnsHash [][]byte, requestType pb.RequestTransactionType) error {
	if len(mergedTxns) != len(txnsHash) {
		return fmt.Errorf("merged txn array len %d is different from txn hash array len %d", len(mergedTxns), len(txnsHash))
	}
	if len(existingTxns)+len(requestedTxns) != len(txnsHash) {
		return fmt.Errorf("sum of existing txn array len %d and requested txn array len %d is different from txn hash array len %d", len(existingTxns), len(requestedTxns), len(txnsHash))
	}

	i, j := 0, 0
	for {
		if i+j == len(mergedTxns) {
			break
		}

		if i < len(existingTxns) {
			txnHashBytes, err := getTxnHash(existingTxns[i], requestType)
			if err != nil {
				return err
			}
			if bytes.Equal(txnHashBytes, txnsHash[i+j]) {
				mergedTxns[i+j] = existingTxns[i]
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

		return errors.New("merge txn error: hash mismatch")
	}

	return nil
}

func (consensus *Consensus) requestProposalTransactions(neighbor *node.RemoteNode, blockHash common.Uint256, requestType pb.RequestTransactionType, txnsHash [][]byte) ([]*transaction.Transaction, error) {
	var shortHashSalt []byte
	var shortHashSize uint32
	if requestType == pb.RequestTransactionType_REQUEST_TRANSACTION_SHORT_HASH {
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
		return nil, fmt.Errorf("returned txn count %d is different from requested count %d", len(replyMsg.Transactions), len(txnsHash))
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
			return nil, fmt.Errorf("the %dth txn hash %x is different from expected %x", i, txnHash, txnsHash[i])
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
			log.Warningf("Send message to neighbor %v error: %v", neighbor, err)
			continue
		}
	}

	return nil
}
