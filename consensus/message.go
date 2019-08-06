package consensus

import (
	"crypto/sha256"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/block"
	"github.com/nknorg/nkn/chain"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/transaction"
)

// NewVoteMessage creates a VOTE message
func NewVoteMessage(height uint32, blockHash common.Uint256) (*pb.UnsignedMessage, error) {
	msgBody := &pb.Vote{
		Height:    height,
		BlockHash: blockHash[:],
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.VOTE,
		Message:     buf,
	}

	return msg, nil
}

// NewIHaveBlockProposalMessage creates a I_HAVE_BLOCK_PROPOSAL message
func NewIHaveBlockProposalMessage(height uint32, blockHash common.Uint256) (*pb.UnsignedMessage, error) {
	msgBody := &pb.IHaveBlockProposal{
		Height:    height,
		BlockHash: blockHash[:],
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.I_HAVE_BLOCK_PROPOSAL,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestBlockProposalMessage creates a REQUEST_BLOCK_PROPOSAL message to
// request a block
func NewRequestBlockProposalMessage(blockHash common.Uint256, requestType pb.RequestTransactionType, shortHashSalt []byte, shortHashSize uint32) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestBlockProposal{
		BlockHash:     blockHash.ToArray(),
		Type:          requestType,
		ShortHashSalt: shortHashSalt,
		ShortHashSize: shortHashSize,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_BLOCK_PROPOSAL,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestBlockProposalReply creates a REQUEST_BLOCK_PROPOSAL_REPLY message
// in respond to REQUEST_BLOCK_PROPOSAL message to send a block
func NewRequestBlockProposalReply(b *block.Block, txnsHash [][]byte) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestBlockProposalReply{
		Block:            b.ToMsgBlock(),
		TransactionsHash: txnsHash,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_BLOCK_PROPOSAL_REPLY,
		Message:     buf,
	}

	return msg, nil
}

// NewGetConsensusStateMessage creates a GET_CONSENSUS_STATE message
func NewGetConsensusStateMessage() (*pb.UnsignedMessage, error) {
	msgBody := &pb.GetConsensusState{}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.GET_CONSENSUS_STATE,
		Message:     buf,
	}

	return msg, nil
}

// NewGetConsensusStateReply creates a GET_CONSENSUS_STATE_REPLY message in
// respond to GET_CONSENSUS_STATE message
func NewGetConsensusStateReply(ledgerBlockHash common.Uint256, ledgerHeight, consensusHeight, minVerifiableHeight uint32, syncState pb.SyncState) (*pb.UnsignedMessage, error) {
	msgBody := &pb.GetConsensusStateReply{
		LedgerBlockHash:     ledgerBlockHash[:],
		LedgerHeight:        ledgerHeight,
		ConsensusHeight:     consensusHeight,
		MinVerifiableHeight: minVerifiableHeight,
		SyncState:           syncState,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.GET_CONSENSUS_STATE_REPLY,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestProposalTransactionsMessage creates a REQUEST_PROPOSAL_TRANSACTIONS
// message to request a block
func NewRequestProposalTransactionsMessage(blockHash common.Uint256, requestType pb.RequestTransactionType, shortHashSalt []byte, shortHashSize uint32, transactionsHash [][]byte) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestProposalTransactions{
		BlockHash:        blockHash.ToArray(),
		Type:             requestType,
		ShortHashSalt:    shortHashSalt,
		ShortHashSize:    shortHashSize,
		TransactionsHash: transactionsHash,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_PROPOSAL_TRANSACTIONS,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestProposalTransactionsReply creates a
// REQUEST_PROPOSAL_TRANSACTIONS_REPLY message in respond to
// REQUEST_PROPOSAL_TRANSACTIONS message to send a block
func NewRequestProposalTransactionsReply(transactions []*transaction.Transaction) (*pb.UnsignedMessage, error) {
	msgTxns := make([]*pb.Transaction, len(transactions))
	for i, txn := range transactions {
		msgTxns[i] = txn.Transaction
	}

	msgBody := &pb.RequestProposalTransactionsReply{
		Transactions: msgTxns,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_PROPOSAL_TRANSACTIONS_REPLY,
		Message:     buf,
	}

	return msg, nil
}

// voteMessageHandler handles a VOTE message
func (consensus *Consensus) voteMessageHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {
	msgBody := &pb.Vote{}
	err := proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return nil, false, err
	}

	blockHash, err := common.Uint256ParseFromBytes(msgBody.BlockHash)
	if err != nil {
		return nil, false, err
	}

	err = consensus.receiveVote(remoteMessage.Sender.GetID(), msgBody.Height, blockHash)
	if err != nil {
		return nil, false, err
	}

	return nil, false, nil
}

// iHaveBlockProposalMessageHandler handles a I_HAVE_BLOCK_PROPOSAL message
func (consensus *Consensus) iHaveBlockProposalMessageHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {
	msgBody := &pb.IHaveBlockProposal{}
	err := proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return nil, false, err
	}

	blockHash, err := common.Uint256ParseFromBytes(msgBody.BlockHash)
	if err != nil {
		return nil, false, err
	}

	err = consensus.receiveProposalHash(remoteMessage.Sender.GetID(), msgBody.Height, blockHash)
	if err != nil {
		return nil, false, err
	}

	return nil, false, nil
}

// getConsensusStateMessageHandler handles a GET_CONSENSUS_STATE message
func (consensus *Consensus) getConsensusStateMessageHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {
	ledgerHeight := chain.DefaultLedger.Store.GetHeight()
	ledgerBlockHash := chain.DefaultLedger.Store.GetHeaderHashByHeight(ledgerHeight)
	consensusHeight := consensus.GetExpectedHeight()
	syncState := consensus.localNode.GetSyncState()
	minVerifiableHeight := consensus.localNode.GetMinVerifiableHeight()

	replyMsg, err := NewGetConsensusStateReply(ledgerBlockHash, ledgerHeight, consensusHeight, minVerifiableHeight, syncState)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := consensus.localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

// requestBlockProposalMessageHandler handles a REQUEST_BLOCK_PROPOSAL message
func (consensus *Consensus) requestBlockProposalMessageHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {
	replyMsg, err := NewRequestBlockProposalReply(nil, nil)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := consensus.localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return nil, false, err
	}

	msgBody := &pb.RequestBlockProposal{}
	err = proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return replyBuf, false, err
	}

	blockHash, err := common.Uint256ParseFromBytes(msgBody.BlockHash)
	if err != nil {
		return replyBuf, false, err
	}

	fullBlock, err := consensus.getBlockProposal(blockHash)
	if err != nil {
		return replyBuf, false, err
	}

	var b *block.Block
	var txnsHash [][]byte

	switch msgBody.Type {
	case pb.REQUEST_FULL_TRANSACTION:
		b = fullBlock
	case pb.REQUEST_TRANSACTION_HASH:
		b = &block.Block{Header: fullBlock.Header}
		txnsHash = make([][]byte, len(fullBlock.Transactions))
		for i, txn := range fullBlock.Transactions {
			txnHash := txn.Hash()
			txnsHash[i] = txnHash.ToArray()
		}
	case pb.REQUEST_TRANSACTION_SHORT_HASH:
		if msgBody.ShortHashSize > sha256.Size {
			return replyBuf, false, fmt.Errorf("hash size %d is greater than %d", msgBody.ShortHashSize, sha256.Size)
		}
		b = &block.Block{Header: fullBlock.Header}
		txnsHash = make([][]byte, len(fullBlock.Transactions))
		for i, txn := range fullBlock.Transactions {
			txnsHash[i] = txn.ShortHash(msgBody.ShortHashSalt, msgBody.ShortHashSize)
		}
	}

	replyMsg, err = NewRequestBlockProposalReply(b, txnsHash)
	if err != nil {
		return replyBuf, false, err
	}

	replyBuf, err = consensus.localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

// requestProposalTransactionsMessageHandler handles a REQUEST_PROPOSAL_TRANSACTIONS message
func (consensus *Consensus) requestProposalTransactionsMessageHandler(remoteMessage *node.RemoteMessage) ([]byte, bool, error) {
	replyMsg, err := NewRequestProposalTransactionsReply(nil)
	if err != nil {
		return nil, false, err
	}

	replyBuf, err := consensus.localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return nil, false, err
	}

	msgBody := &pb.RequestProposalTransactions{}
	err = proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return replyBuf, false, err
	}

	value, ok := consensus.proposals.Get(msgBody.BlockHash)
	if !ok {
		return replyBuf, false, fmt.Errorf("Block %x not found in local cache", msgBody.BlockHash)
	}

	b, ok := value.(*block.Block)
	if !ok {
		return replyBuf, false, fmt.Errorf("Convert block %x from proposal cache error", msgBody.BlockHash)
	}

	if len(msgBody.TransactionsHash) > len(b.Transactions) {
		return replyBuf, false, fmt.Errorf("Requested txn count %d is greater than block txn count %d", len(msgBody.TransactionsHash), len(b.Transactions))
	}

	txns := make([]*transaction.Transaction, len(msgBody.TransactionsHash))

	switch msgBody.Type {
	case pb.REQUEST_TRANSACTION_HASH:
		txnMap := make(map[common.Uint256]*transaction.Transaction)
		for _, txn := range b.Transactions {
			txnMap[txn.Hash()] = txn
		}

		for i, txnHashBytes := range msgBody.TransactionsHash {
			var txnHash common.Uint256
			txnHash, err = common.Uint256ParseFromBytes(txnHashBytes)
			if err != nil {
				return replyBuf, false, err
			}
			txns[i], ok = txnMap[txnHash]
			if !ok {
				return replyBuf, false, fmt.Errorf("Transaction %s not found in block", txnHash.ToHexString())
			}
		}
	case pb.REQUEST_TRANSACTION_SHORT_HASH:
		txnMap := make(map[string]*transaction.Transaction)
		for _, txn := range b.Transactions {
			txnMap[string(txn.ShortHash(msgBody.ShortHashSalt, msgBody.ShortHashSize))] = txn
		}

		for i := range msgBody.TransactionsHash {
			txns[i], ok = txnMap[string(msgBody.TransactionsHash[i])]
			if !ok {
				return replyBuf, false, fmt.Errorf("Transaction %x not found in block", msgBody.TransactionsHash[i])
			}
		}
	}

	replyMsg, err = NewRequestProposalTransactionsReply(txns)
	if err != nil {
		return replyBuf, false, err
	}

	replyBuf, err = consensus.localNode.SerializeMessage(replyMsg, false)
	return replyBuf, false, err
}

func (consensus *Consensus) registerMessageHandler() {
	consensus.localNode.AddMessageHandler(pb.VOTE, consensus.voteMessageHandler)
	consensus.localNode.AddMessageHandler(pb.I_HAVE_BLOCK_PROPOSAL, consensus.iHaveBlockProposalMessageHandler)
	consensus.localNode.AddMessageHandler(pb.GET_CONSENSUS_STATE, consensus.getConsensusStateMessageHandler)
	consensus.localNode.AddMessageHandler(pb.REQUEST_BLOCK_PROPOSAL, consensus.requestBlockProposalMessageHandler)
	consensus.localNode.AddMessageHandler(pb.REQUEST_PROPOSAL_TRANSACTIONS, consensus.requestProposalTransactionsMessageHandler)
}
