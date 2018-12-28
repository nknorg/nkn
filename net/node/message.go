package node

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/net/message"
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

func (node *node) SerializeMessage(unsignedMsg *pb.UnsignedMessage, sign bool) ([]byte, error) {
	if node.account == nil {
		return nil, errors.New("Account is nil")
	}

	buf, err := proto.Marshal(unsignedMsg)
	if err != nil {
		return nil, err
	}

	var signature []byte
	if sign {
		hash := sha256.Sum256(buf)
		signature, err = crypto.Sign(node.account.PrivateKey, hash[:])
		if err != nil {
			return nil, err
		}
	}

	signedMsg := &pb.SignedMessage{
		Message:   buf,
		Signature: signature,
	}

	return proto.Marshal(signedMsg)
}

func (node *node) SendBytesAsync(buf []byte) error {
	err := node.local.nnet.SendBytesDirectAsync(buf, node.nnetNode)
	if err != nil {
		log.Errorf("Error sending async messge to node %v, removing node.", err.Error())
		node.CloseConn()
		node.local.DelNbrNode(node.GetID())
	}
	return err
}

func (node *node) SendBytesSync(buf []byte) ([]byte, error) {
	reply, _, err := node.local.nnet.SendBytesDirectSync(buf, node.nnetNode)
	if err != nil {
		log.Errorf("Error sending sync messge to node %v", err.Error())
	}
	return reply, err
}

func (node *node) SendBytesReply(replyToID, buf []byte) error {
	err := node.local.nnet.SendBytesDirectReply(replyToID, buf, node.nnetNode)
	if err != nil {
		log.Errorf("Error sending async messge to node %v, removing node.", err.Error())
		node.CloseConn()
		node.local.DelNbrNode(node.GetID())
	}
	return err
}

func handleMessage(sender protocol.Noder, signedMsg *pb.SignedMessage, routingType nnetpb.RoutingType) ([]byte, error) {
	signed := false
	if len(signedMsg.Signature) > 0 && routingType == nnetpb.DIRECT {
		pubKey := sender.GetPubKey()
		if pubKey == nil {
			return nil, errors.New("Neighbor public key is nil")
		}

		hash := sha256.Sum256(signedMsg.Message)
		err := crypto.Verify(*pubKey, hash[:], signedMsg.Signature)
		if err != nil {
			return nil, err
		}
		signed = true
	}

	unsignedMsg := &pb.UnsignedMessage{}
	err := proto.Unmarshal(signedMsg.Message, unsignedMsg)
	if err != nil {
		return nil, err
	}

	// whitelist message type that can be unsigned
	switch unsignedMsg.MessageType {
	default:
		if !signed {
			return nil, fmt.Errorf("message of type %v should be signed", unsignedMsg.MessageType)
		}
	}

	// whitelist allowed message type for each routing type
	switch routingType {
	case nnetpb.DIRECT:
		switch unsignedMsg.MessageType {
		case pb.VOTE:
		case pb.I_HAVE_BLOCK:
		case pb.REQUEST_BLOCK:
		case pb.GET_CONSENSUS_STATE:
		default:
			return nil, fmt.Errorf("message of type %v is not allowed for routing type %v", unsignedMsg.MessageType, routingType)
		}
	case nnetpb.RELAY:
		switch unsignedMsg.MessageType {
		default:
			return nil, fmt.Errorf("message of type %v is not allowed for routing type %v", unsignedMsg.MessageType, routingType)
		}
	case nnetpb.BROADCAST_PUSH:
		switch unsignedMsg.MessageType {
		default:
			return nil, fmt.Errorf("message of type %v is not allowed for routing type %v", unsignedMsg.MessageType, routingType)
		}
	default:
		return nil, fmt.Errorf("message of type %v is not allowed for routing type %v", unsignedMsg.MessageType, routingType)
	}

	remoteMessage := &message.RemoteMessage{
		Sender:  sender,
		Message: unsignedMsg.Message,
	}

	var reply []byte
	var shouldCallNext bool

	for _, handler := range message.GetHandlers(unsignedMsg.MessageType) {
		reply, shouldCallNext, err = handler(remoteMessage)
		if err != nil {
			log.Errorf("Get error when handling message: %v", err)
			continue
		}

		if !shouldCallNext {
			break
		}
	}

	return reply, nil
}
