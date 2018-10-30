package node

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/message"
	"github.com/nknorg/nnet/protobuf"
)

const (
	// msg length is encoded by 32 bit int
	msgLenBytes = 4
)

// RemoteMessage is the received msg from remote node. RemoteNode is nil if
// message is sent by local node.
type RemoteMessage struct {
	RemoteNode *RemoteNode
	Msg        *protobuf.Message
}

// NewRemoteMessage creates a RemoteMessage with remote node rn and msg
func NewRemoteMessage(rn *RemoteNode, msg *protobuf.Message) (*RemoteMessage, error) {
	remoteMsg := &RemoteMessage{
		RemoteNode: rn,
		Msg:        msg,
	}
	return remoteMsg, nil
}

// NewPingMessage creates a PING message for heartbeat
func NewPingMessage() (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.Ping{}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.PING,
		RoutingType: protobuf.DIRECT,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// NewPingReply creates a PING reply for heartbeat
func NewPingReply(replyToID []byte) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.PingReply{}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.PING,
		RoutingType: protobuf.DIRECT,
		ReplyToId:   replyToID,
		MessageId:   id,
		Message:     buf,
	}
	return msg, nil
}

// NewGetNodeMessage creates a GET_NODE message to get node info
func NewGetNodeMessage() (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.GetNode{}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.GET_NODE,
		RoutingType: protobuf.DIRECT,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// NewGetNodeReply creates a GET_NODE reply to send node info
func NewGetNodeReply(replyToID []byte, n *protobuf.Node) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.GetNodeReply{
		Node: n,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.GET_NODE,
		RoutingType: protobuf.DIRECT,
		ReplyToId:   replyToID,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// NewStopMessage creates a STOP message to notify local node to close
// connection with remote node
func NewStopMessage() (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.Stop{}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.STOP,
		RoutingType: protobuf.DIRECT,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// handleRemoteMessage handles a remote message and returns error
func (ln *LocalNode) handleRemoteMessage(remoteMsg *RemoteMessage) error {
	if remoteMsg.RemoteNode == nil && remoteMsg.Msg.MessageType != protobuf.BYTES {
		return errors.New("Message is sent by local node")
	}

	switch remoteMsg.Msg.MessageType {
	case protobuf.PING:
		replyMsg, err := NewPingReply(remoteMsg.Msg.MessageId)
		if err != nil {
			return err
		}

		err = remoteMsg.RemoteNode.SendMessageAsync(replyMsg)
		if err != nil {
			return err
		}

	case protobuf.GET_NODE:
		replyMsg, err := NewGetNodeReply(remoteMsg.Msg.MessageId, remoteMsg.RemoteNode.LocalNode.Node.Node)
		if err != nil {
			return err
		}

		err = remoteMsg.RemoteNode.SendMessageAsync(replyMsg)
		if err != nil {
			return err
		}

	case protobuf.STOP:
		log.Infof("Received stop message from remote node %v", remoteMsg.RemoteNode)
		remoteMsg.RemoteNode.Stop(nil)

	case protobuf.BYTES:
		msgBody := &protobuf.Bytes{}
		err := proto.Unmarshal(remoteMsg.Msg.Message, msgBody)
		if err != nil {
			return err
		}

		data := msgBody.Data
		var shouldCallNextMiddleware bool
		for _, f := range ln.middlewareStore.bytesReceived {
			data, shouldCallNextMiddleware = f(data, remoteMsg.Msg.MessageId, remoteMsg.Msg.SrcId, remoteMsg.RemoteNode)
			if !shouldCallNextMiddleware {
				break
			}
		}

	default:
		return errors.New("Unknown message type")
	}

	return nil
}
