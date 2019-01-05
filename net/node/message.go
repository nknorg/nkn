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
	nnetnode "github.com/nknorg/nnet/node"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

func (node *Node) SerializeMessage(unsignedMsg *pb.UnsignedMessage, sign bool) ([]byte, error) {
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

func (node *Node) SendBytesAsync(buf []byte) error {
	err := node.local.nnet.SendBytesDirectAsync(buf, node.nnetNode)
	if err != nil {
		log.Errorf("Error sending async messge to node %v, removing node.", err.Error())
		node.CloseConn()
		node.local.DelNbrNode(node.GetID())
	}
	return err
}

func (node *Node) SendBytesSync(buf []byte) ([]byte, error) {
	reply, _, err := node.local.nnet.SendBytesDirectSync(buf, node.nnetNode)
	if err != nil {
		log.Errorf("Error sending sync messge to node: %v", err.Error())
	}
	return reply, err
}

func (node *Node) SendBytesReply(replyToID, buf []byte) error {
	err := node.local.nnet.SendBytesDirectReply(replyToID, buf, node.nnetNode)
	if err != nil {
		log.Errorf("Error sending async messge to node: %v, removing node.", err.Error())
		node.CloseConn()
		node.local.DelNbrNode(node.GetID())
	}
	return err
}

func (node *Node) remoteMessageRouted(remoteMessage *nnetnode.RemoteMessage, localNode *nnetnode.LocalNode, remoteNodes []*nnetnode.RemoteNode) (*nnetnode.RemoteMessage, *nnetnode.LocalNode, []*nnetnode.RemoteNode, bool) {
	if remoteMessage.Msg.MessageType == nnetpb.BYTES {
		err := node.maybeAddRemoteNode(remoteMessage.RemoteNode)
		if err != nil {
			log.Errorf("Add remote node error: %v", err)
		}

		for _, remoteNode := range remoteNodes {
			err = node.maybeAddRemoteNode(remoteNode)
			if err != nil {
				log.Errorf("Add remote node error: %v", err)
			}
		}

		msgBody := &nnetpb.Bytes{}
		err = proto.Unmarshal(remoteMessage.Msg.Message, msgBody)
		if err != nil {
			log.Errorf("Error unmarshal byte msg: %v", err)
			return nil, nil, nil, false
		}

		signedMsg := &pb.SignedMessage{}
		unsignedMsg := &pb.UnsignedMessage{}
		err = proto.Unmarshal(msgBody.Data, signedMsg)
		if err != nil {
			// TODO: remove this part after all msg are migrated to pb
			signedMsg = nil
			unsignedMsg = nil
		} else {
			err = proto.Unmarshal(signedMsg.Message, unsignedMsg)
			if err != nil {
				log.Errorf("Error unmarshal signed unsigned msg: %v", err)
				return nil, nil, nil, false
			}

			err = message.CheckMessageType(unsignedMsg.MessageType)
			if err != nil {
				log.Errorf("Error checking message type: %v", err)
				return nil, nil, nil, false
			}

			err = message.CheckSigned(unsignedMsg.MessageType, len(signedMsg.Signature) > 0)
			if err != nil {
				log.Errorf("Error checking signed: %v", err)
				return nil, nil, nil, false
			}

			err = message.CheckRoutingType(unsignedMsg.MessageType, remoteMessage.Msg.RoutingType)
			if err != nil {
				log.Errorf("Error checking routing type: %v", err)
				return nil, nil, nil, false
			}

			if len(signedMsg.Signature) > 0 && remoteMessage.Msg.RoutingType != nnetpb.DIRECT {
				log.Errorf("Signature is only allowed on direct message")
				return nil, nil, nil, false
			}
		}

		if localNode != nil {
			sender := node.getNbrByNNetNode(remoteMessage.RemoteNode)
			if sender == nil {
				log.Error("Cannot get neighbor node")
				return nil, nil, nil, false
			}

			if signedMsg == nil {
				// TODO: remove this part after all msg are migrated to pb
				err = message.HandleNodeMsg(sender, msgBody.Data)
				if err != nil {
					log.Errorf("Error handling node msg: %v", err)
					return nil, nil, nil, false
				}
				localNode = nil
			} else {
				if len(signedMsg.Signature) > 0 {
					pubKey := sender.GetPubKey()
					if pubKey == nil {
						log.Errorf("Neighbor public key is nil")
						return nil, nil, nil, false
					}

					hash := sha256.Sum256(signedMsg.Message)
					err := crypto.Verify(*pubKey, hash[:], signedMsg.Signature)
					if err != nil {
						log.Errorf("Verify signature error: %v", err)
						return nil, nil, nil, false
					}
				}

				if len(remoteMessage.Msg.ReplyToId) == 0 {
					reply, err := node.receiveMessage(sender, unsignedMsg)
					if err != nil {
						log.Errorf("Error handling message: %v", err)
						return nil, nil, nil, false
					}

					if len(reply) > 0 {
						err = sender.SendBytesReply(remoteMessage.Msg.MessageId, reply)
						if err != nil {
							log.Errorf("Error sending reply: %v", err)
							return nil, nil, nil, false
						}
					}

					localNode = nil
				} else {
					msgBody.Data = unsignedMsg.Message
					remoteMessage.Msg.Message, err = proto.Marshal(msgBody)
					if err != nil {
						log.Errorf("Marshal reply msg body error: %v", err)
						return nil, nil, nil, false
					}
				}
			}
		}

		if localNode == nil && len(remoteNodes) == 0 {
			return nil, nil, nil, false
		}

		if remoteMessage.Msg.RoutingType == nnetpb.RELAY && len(remoteNodes) > 0 {
			msg, err := message.ParseMsg(msgBody.Data)
			if err != nil {
				log.Errorf("Parse msg error: %v", err)
				return nil, nil, nil, false
			}

			relayMsg, ok := msg.(*message.RelayMessage)
			if !ok {
				log.Errorf("Msg is not relay message")
				return nil, nil, nil, false
			}

			relayMsg, err = node.processRelayMessage(relayMsg, remoteNodes)
			if err != nil {
				log.Errorf("Process relay msg error: %v", err)
				return nil, nil, nil, false
			}

			msgBody.Data, err = relayMsg.ToBytes()
			if err != nil {
				log.Errorf("Relay msg to bytes error: %v", err)
				return nil, nil, nil, false
			}

			remoteMessage.Msg.Message, err = proto.Marshal(msgBody)
			if err != nil {
				log.Errorf("Marshal relay msg body error: %v", err)
				return nil, nil, nil, false
			}
		}
	}

	return remoteMessage, localNode, remoteNodes, true
}

func (node *Node) receiveMessage(sender protocol.Noder, unsignedMsg *pb.UnsignedMessage) ([]byte, error) {
	remoteMessage := &RemoteMessage{
		Sender:  sender,
		Message: unsignedMsg.Message,
	}

	var reply []byte
	var shouldCallNext bool
	var err error

	for _, handler := range node.GetHandlers(unsignedMsg.MessageType) {
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

func (node *Node) processRelayMessage(relayMsg *message.RelayMessage, remoteNodes []*nnetnode.RemoteNode) (*message.RelayMessage, error) {
	if len(remoteNodes) == 0 {
		return nil, fmt.Errorf("no next hop")
	}

	if len(remoteNodes) > 1 {
		return nil, fmt.Errorf("multiple next hop is not supported yet")
	}

	nextHop := node.getNbrByNNetNode(remoteNodes[0])
	if nextHop == nil {
		return nil, fmt.Errorf("cannot get next hop neighbor node")
	}

	relayPacket := &relayMsg.Packet
	err := node.relayer.SignRelayPacket(nextHop, relayPacket)
	if err != nil {
		return nil, err
	}

	relayMsg, err = message.NewRelayMessage(relayPacket)
	if err != nil {
		return nil, err
	}

	return relayMsg, nil
}
