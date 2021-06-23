package node

import (
	"errors"

	"github.com/nknorg/nkn/v2/chain/trie"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/chain/store"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
)

var (
	errInvalidAmount = errors.New("invalid states data length")
	errInvalidData   = errors.New("invalid state data")
)

// NewGetStateMessage creates a GET_STATES message
func NewGetStateMessage(hashes []common.Uint256) (*pb.UnsignedMessage, error) {
	msgStates := make([]*pb.StateRequest, len(hashes))
	for i := range hashes {
		msgStates[i] = &pb.StateRequest{Hash: hashes[i].ToArray()}
	}
	states := &pb.GetStates{
		Reqs: msgStates,
	}

	buf, err := proto.Marshal(states)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_GET_STATES,
		Message:     buf,
	}

	return msg, nil
}

// NewGetStatesReply creates a GET_STATES_REPLY message in respond to GET_STATES
// message
func NewGetStatesReply(nodes [][]byte) (*pb.UnsignedMessage, error) {
	msgNodes := make([]*pb.StateNode, len(nodes))
	for i, n := range nodes {
		msgNodes[i] = &pb.StateNode{Node: n}
	}

	msgBody := &pb.GetStatesReply{
		Nodes: msgNodes,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &pb.UnsignedMessage{
		MessageType: pb.MessageType_GET_STATES_REPLY,
		Message:     buf,
	}

	return msg, nil
}

func (localNode *LocalNode) getStatesMessageHandler(remoteMessage *RemoteMessage) ([]byte, bool, error) {
	replyMsg, err := NewGetStatesReply(nil)
	if err != nil {
		return nil, false, err
	}
	replyBuf, err := localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return nil, false, err
	}
	msgBody := &pb.GetStates{}
	err = proto.Unmarshal(remoteMessage.Message, msgBody)
	if err != nil {
		return replyBuf, false, err
	}

	sdb := chain.DefaultLedger.Store.(*store.ChainStore).States
	nodes := make([][]byte, len(msgBody.Reqs))
	for i, req := range msgBody.Reqs {
		h, err := common.Uint256ParseFromBytes(req.Hash)
		if err != nil {
			return replyBuf, false, err
		}
		enc, err := sdb.GetState(h)
		if err != nil {
			return replyBuf, false, err
		}
		if len(enc) == 0 {
			return replyBuf, false, errors.New("no states found")
		}
		nodes[i] = enc
	}
	replyMsg, err = NewGetStatesReply(nodes)
	if err != nil {
		return replyBuf, false, err
	}
	replyBuf, err = localNode.SerializeMessage(replyMsg, false)
	if err != nil {
		return replyBuf, false, err
	}

	return replyBuf, false, nil
}

func (remoteNode *RemoteNode) GetStates(hashes []common.Uint256) ([][]byte, error) {
	msg, err := NewGetStateMessage(hashes)
	if err != nil {
		return nil, err
	}
	buf, err := remoteNode.localNode.SerializeMessage(msg, false)
	if err != nil {
		return nil, err
	}
	replyBytes, err := remoteNode.SendBytesSyncWithTimeout(buf, syncReplyTimeout)
	if err != nil {
		return nil, err
	}
	replyMsg := &pb.GetStatesReply{}
	err = proto.Unmarshal(replyBytes, replyMsg)
	if err != nil {
		return nil, err
	}
	states := make([][]byte, len(hashes))
	nodes := replyMsg.GetNodes()
	if len(nodes) != len(hashes) {
		return nil, errInvalidAmount
	}
	for i, s := range nodes {
		h, err := common.Uint256ParseFromBytes(trie.Sha256Key(s.Node))
		if err != nil {
			return nil, err
		}
		if !common.U256Equal(h, hashes[i]) {
			return nil, errInvalidData
		}
		states[i] = s.GetNode()
	}
	return states, nil
}
