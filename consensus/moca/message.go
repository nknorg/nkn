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

// NewIHaveBlockMessage creates a I_HAVE_BLOCK message
func NewIHaveBlockMessage(height uint32, blockHash common.Uint256) (*pb.UnsignedMessage, error) {
	msgBody := &pb.IHaveBlock{
		Height:    height,
		BlockHash: blockHash[:],
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.I_HAVE_BLOCK,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestBlockMessage creates a REQUEST_BLOCK message to request a block
func NewRequestBlockMessage(blockHash common.Uint256) (*pb.UnsignedMessage, error) {
	msgBody := &pb.RequestBlock{
		BlockHash: blockHash[:],
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.REQUEST_BLOCK,
		Message:     buf,
	}

	return msg, nil
}

// NewRequestBlockReply creates a REQUEST_BLOCK reply to send a block
func NewRequestBlockReply(block *ledger.Block) (*pb.RequestBlockReply, error) {
	var buf []byte
	if block != nil {
		b := new(bytes.Buffer)
		block.Serialize(b)
		buf = b.Bytes()
	}

	msg := &pb.RequestBlockReply{
		Block: buf,
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
func NewGetConsensusStateReply(ledgerHeight uint32, ledgerBlockHash common.Uint256, consensusHeight uint32, syncState pb.SyncState) (*pb.GetConsensusStateReply, error) {
	msg := &pb.GetConsensusStateReply{
		LedgerHeight:    ledgerHeight,
		LedgerBlockHash: ledgerBlockHash[:],
		ConsensusHeight: consensusHeight,
		SyncState:       syncState,
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

	message.AddHandler(pb.I_HAVE_BLOCK, func(remoteMessage *message.RemoteMessage) ([]byte, bool, error) {
		msgBody := &pb.IHaveBlock{}
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

	message.AddHandler(pb.REQUEST_BLOCK, func(remoteMessage *message.RemoteMessage) ([]byte, bool, error) {
		replyMsg, err := NewRequestBlockReply(nil)
		if err != nil {
			return nil, false, err
		}

		replyBuf, err := proto.Marshal(replyMsg)
		if err != nil {
			return nil, false, err
		}

		msgBody := &pb.RequestBlock{}
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

		replyMsg, err = NewRequestBlockReply(block)
		if err != nil {
			return replyBuf, false, err
		}

		replyBuf, err = proto.Marshal(replyMsg)
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

		replyBuf, err := proto.Marshal(replyMsg)
		return replyBuf, false, err
	})
}
