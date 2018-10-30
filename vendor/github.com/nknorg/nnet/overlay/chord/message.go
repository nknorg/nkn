package chord

import (
	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet/message"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/protobuf"
)

// NewGetSuccAndPredMessage creates a GET_SUCC_AND_PRED message to get the
// successors and predecessor of a remote node
func NewGetSuccAndPredMessage(numSucc, numPred uint32) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.GetSuccAndPred{
		NumSucc: numSucc,
		NumPred: numPred,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.GET_SUCC_AND_PRED,
		RoutingType: protobuf.DIRECT,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// NewGetSuccAndPredReply creates a GET_SUCC_AND_PRED reply to send successors
// and predecessor
func NewGetSuccAndPredReply(replyToID []byte, successors, predecessors []*protobuf.Node) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.GetSuccAndPredReply{
		Successors:   successors,
		Predecessors: predecessors,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.GET_SUCC_AND_PRED,
		RoutingType: protobuf.DIRECT,
		ReplyToId:   replyToID,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// NewFindSuccAndPredMessage creates a FIND_SUCC_AND_PRED message to find
// numSucc successors and numPred predecessors of a key
func NewFindSuccAndPredMessage(key []byte, numSucc, numPred uint32) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.FindSuccAndPred{
		Key:     key,
		NumSucc: numSucc,
		NumPred: numPred,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.FIND_SUCC_AND_PRED,
		RoutingType: protobuf.DIRECT,
		MessageId:   id,
		Message:     buf,
		DestId:      key,
	}

	return msg, nil
}

// NewFindSuccAndPredReply creates a FIND_SUCC_AND_PRED reply to send successors
// and predecessors
func NewFindSuccAndPredReply(replyToID []byte, successors, predecessors []*protobuf.Node) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.FindSuccAndPredReply{
		Successors:   successors,
		Predecessors: predecessors,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.FIND_SUCC_AND_PRED,
		RoutingType: protobuf.DIRECT,
		ReplyToId:   replyToID,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// handleRemoteMessage handles a remote message and returns if it should be
// passed through to local node and error
func (c *Chord) handleRemoteMessage(remoteMsg *node.RemoteMessage) (bool, error) {
	if remoteMsg.RemoteNode == nil {
		return true, nil
	}

	switch remoteMsg.Msg.MessageType {
	case protobuf.GET_SUCC_AND_PRED:
		msgBody := &protobuf.GetSuccAndPred{}
		err := proto.Unmarshal(remoteMsg.Msg.Message, msgBody)
		if err != nil {
			return false, err
		}

		succs := c.successors.ToProtoNodeList(true)
		if succs != nil && uint32(len(succs)) > msgBody.NumSucc {
			succs = succs[:msgBody.NumSucc]
		}

		preds := c.predecessors.ToProtoNodeList(true)
		if preds != nil && uint32(len(preds)) > msgBody.NumPred {
			preds = preds[:msgBody.NumPred]
		}

		replyMsg, err := NewGetSuccAndPredReply(remoteMsg.Msg.MessageId, succs, preds)
		if err != nil {
			return false, err
		}

		err = remoteMsg.RemoteNode.SendMessageAsync(replyMsg)
		if err != nil {
			return false, err
		}

	case protobuf.FIND_SUCC_AND_PRED:
		msgBody := &protobuf.FindSuccAndPred{}
		err := proto.Unmarshal(remoteMsg.Msg.Message, msgBody)
		if err != nil {
			return false, err
		}

		succs, preds, err := c.FindSuccAndPred(msgBody.Key, msgBody.NumSucc, msgBody.NumPred)
		if err != nil {
			return false, err
		}

		replyMsg, err := NewFindSuccAndPredReply(remoteMsg.Msg.MessageId, succs, preds)
		if err != nil {
			return false, err
		}

		err = remoteMsg.RemoteNode.SendMessageAsync(replyMsg)
		if err != nil {
			return false, err
		}

	default:
		return true, nil
	}

	return false, nil
}
