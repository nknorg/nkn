package moca

import (
	"bytes"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/pb"
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
func NewRequestBlockProposalMessage(blockHash common.Uint256) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestBlockProposal{
		BlockHash: blockHash[:],
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

// NewRequestBlockProposalReply creates a REQUEST_BLOCK_PROPOSAL reply to send a
// block
func NewRequestBlockProposalReply(block *ledger.Block) (*pb.UnsignedMessage, error) {
	var buf []byte
	if block != nil {
		b := new(bytes.Buffer)
		block.Serialize(b)
		buf = b.Bytes()
	}

	msgBody := &pb.RequestBlockProposalReply{
		Block: buf,
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

// NewGetConsensusStateReply creates a GET_CONSENSUS_STATE reply
func NewGetConsensusStateReply(ledgerHeight uint32, ledgerBlockHash common.Uint256, consensusHeight uint32, syncState pb.SyncState) (*pb.UnsignedMessage, error) {
	msgBody := &pb.GetConsensusStateReply{
		LedgerHeight:    ledgerHeight,
		LedgerBlockHash: ledgerBlockHash[:],
		ConsensusHeight: consensusHeight,
		SyncState:       syncState,
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

func (consensus *Consensus) registerMessageHandler() {
	message.AddHandler(pb.VOTE, func(remoteMessage *message.RemoteMessage) ([]byte, bool, error) {
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
	})

	message.AddHandler(pb.I_HAVE_BLOCK_PROPOSAL, func(remoteMessage *message.RemoteMessage) ([]byte, bool, error) {
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
	})

	message.AddHandler(pb.REQUEST_BLOCK_PROPOSAL, func(remoteMessage *message.RemoteMessage) ([]byte, bool, error) {
		replyMsg, err := NewRequestBlockProposalReply(nil)
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

		value, ok := consensus.proposals.Get(blockHash.ToArray())
		if !ok {
			return replyBuf, false, nil
		}

		block, ok := value.(*ledger.Block)
		if !ok {
			return replyBuf, false, nil
		}

		replyMsg, err = NewRequestBlockProposalReply(block)
		if err != nil {
			return replyBuf, false, err
		}

		replyBuf, err = consensus.localNode.SerializeMessage(replyMsg, false)
		return replyBuf, false, err
	})

	message.AddHandler(pb.GET_CONSENSUS_STATE, func(remoteMessage *message.RemoteMessage) ([]byte, bool, error) {
		ledgerHeight := ledger.DefaultLedger.Store.GetHeight()
		ledgerBlockHash := ledger.DefaultLedger.Store.GetHeaderHashByHeight(ledgerHeight)
		consensusHeight := consensus.GetExpectedHeight()
		syncState := consensus.localNode.GetSyncState()

		replyMsg, err := NewGetConsensusStateReply(ledgerHeight, ledgerBlockHash, consensusHeight, syncState)
		if err != nil {
			return nil, false, err
		}

		replyBuf, err := consensus.localNode.SerializeMessage(replyMsg, true)
		return replyBuf, false, err
	})
}
