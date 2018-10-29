package nnet

import (
	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet/message"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/protobuf"
)

// NewDirectBytesMessage creates a BYTES message that send arbitrary bytes to a
// given remote node
func NewDirectBytesMessage(data []byte) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.Bytes{
		Data: data,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.BYTES,
		RoutingType: protobuf.DIRECT,
		MessageId:   id,
		Message:     buf,
	}

	return msg, nil
}

// NewRelayBytesMessage creates a BYTES message that send arbitrary bytes to the
// remote node that has the smallest distance to a given key
func NewRelayBytesMessage(data, srcID, key []byte) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.Bytes{
		Data: data,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.BYTES,
		RoutingType: protobuf.RELAY,
		MessageId:   id,
		Message:     buf,
		SrcId:       srcID,
		DestId:      key,
	}

	return msg, nil
}

// NewBroadcastBytesMessage creates a BYTES message that send arbitrary bytes to
// EVERY remote node in the network (not just neighbors)
func NewBroadcastBytesMessage(data, srcID []byte, routingType protobuf.RoutingType) (*protobuf.Message, error) {
	id, err := message.GenID()
	if err != nil {
		return nil, err
	}

	msgBody := &protobuf.Bytes{
		Data: data,
	}

	buf, err := proto.Marshal(msgBody)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		MessageType: protobuf.BYTES,
		RoutingType: routingType,
		MessageId:   id,
		Message:     buf,
		SrcId:       srcID,
	}

	return msg, nil
}

// SendBytesDirectAsync sends bytes data to a remote node
func (nn *NNet) SendBytesDirectAsync(data []byte, remoteNode *node.RemoteNode) error {
	msg, err := NewDirectBytesMessage(data)
	if err != nil {
		return err
	}

	return remoteNode.SendMessageAsync(msg)
}

// SendBytesDirectSync sends bytes data to a remote node, returns reply message
// or error if timeout
func (nn *NNet) SendBytesDirectSync(data []byte, remoteNode *node.RemoteNode) (*protobuf.Message, error) {
	msg, err := NewDirectBytesMessage(data)
	if err != nil {
		return nil, err
	}

	reply, err := remoteNode.SendMessageSync(msg)
	if err != nil {
		return nil, err
	}

	return reply.Msg, nil
}

// SendBytesDirectReply is the same as SendBytesDirectAsync but with the
// replyToId field of the message set
func (nn *NNet) SendBytesDirectReply(replyToID, data []byte, remoteNode *node.RemoteNode) error {
	msg, err := NewDirectBytesMessage(data)
	if err != nil {
		return err
	}

	msg.ReplyToId = replyToID

	return remoteNode.SendMessageAsync(msg)
}

// SendBytesRelayAsync sends bytes data to the remote node that has smallest
// distance to the key, returns if send success (which is true if successfully
// send message to at least one next hop), and aggregated error during message
// sending
func (nn *NNet) SendBytesRelayAsync(data, key []byte) (bool, error) {
	msg, err := NewRelayBytesMessage(data, nn.GetLocalNode().Id, key)
	if err != nil {
		return false, err
	}

	return nn.SendMessageAsync(msg, protobuf.RELAY)
}

// SendBytesRelaySync sends bytes data to the remote node that has smallest
// distance to the key, returns reply message, if send success (which is true if
// successfully send message to at least one next hop), and aggregated error
// during message sending, will also returns error if doesn't receive any reply
// before timeout
func (nn *NNet) SendBytesRelaySync(data, key []byte) (*protobuf.Message, bool, error) {
	msg, err := NewRelayBytesMessage(data, nn.GetLocalNode().Id, key)
	if err != nil {
		return nil, false, err
	}

	return nn.SendMessageSync(msg, protobuf.RELAY)
}

// SendBytesRelayReply is the same as SendBytesRelayAsync but with the
// replyToId field of the message set
func (nn *NNet) SendBytesRelayReply(replyToID, data, key []byte) (bool, error) {
	msg, err := NewRelayBytesMessage(data, nn.GetLocalNode().Id, key)
	if err != nil {
		return false, err
	}

	msg.ReplyToId = replyToID

	return nn.SendMessageAsync(msg, protobuf.RELAY)
}

// SendBytesBroadcastAsync sends bytes data to EVERY remote node in the network
// (not just neighbors), returns if send success (which is true if successfully
// send message to at least one next hop), and aggregated error during message
// sending
func (nn *NNet) SendBytesBroadcastAsync(data []byte, routingType protobuf.RoutingType) (bool, error) {
	msg, err := NewBroadcastBytesMessage(data, nn.GetLocalNode().Id, routingType)
	if err != nil {
		return false, err
	}

	return nn.SendMessageAsync(msg, routingType)
}

// SendBytesBroadcastSync sends bytes data to EVERY remote node in the network
// (not just neighbors), returns reply message, if send success (which is true
// if successfully send message to at least one next hop), and aggregated error
// during message sending, will also returns error if doesn't receive any reply
// before timeout. Broadcast msg reply should be handled VERY carefully with
// some sort of sender identity verification, otherwise it may be used to DDoS
// attack sender with HUGE amplification factor
func (nn *NNet) SendBytesBroadcastSync(data []byte, routingType protobuf.RoutingType) (*protobuf.Message, bool, error) {
	msg, err := NewBroadcastBytesMessage(data, nn.GetLocalNode().Id, routingType)
	if err != nil {
		return nil, false, err
	}

	return nn.SendMessageSync(msg, routingType)
}

// SendBytesBroadcastReply is the same as SendBytesBroadcastAsync but with the
// replyToId field of the message set. This should NOT be used unless msg src id
// is unknown.
func (nn *NNet) SendBytesBroadcastReply(replyToID, data []byte, routingType protobuf.RoutingType) (bool, error) {
	msg, err := NewBroadcastBytesMessage(data, nn.GetLocalNode().Id, routingType)
	if err != nil {
		return false, err
	}

	msg.ReplyToId = replyToID

	return nn.SendMessageAsync(msg, routingType)
}
