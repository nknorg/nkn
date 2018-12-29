package message

import (
	"fmt"

	"github.com/nknorg/nkn/pb"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

// CheckMessageType checks if a message type is allowed
func CheckMessageType(messageType pb.MessageType) error {
	if messageType == pb.MESSAGE_TYPE_PLACEHOLDER_DO_NOT_USE {
		return fmt.Errorf("message type %s should not be used", pb.MESSAGE_TYPE_PLACEHOLDER_DO_NOT_USE.String())
	}

	return nil
}

// CheckSigned checks if a message type is signed or unsigned as allowed
func CheckSigned(messageType pb.MessageType, signed bool) error {
	switch signed {
	case true:
		if _, ok := pb.AllowedSignedMessageType_name[int32(messageType)]; !ok {
			return fmt.Errorf("message of type %s should be signed", messageType.String())
		}
	case false:
		if _, ok := pb.AllowedUnsignedMessageType_name[int32(messageType)]; !ok {
			return fmt.Errorf("message of type %s should not be signed", messageType.String())
		}
	}
	return nil
}

// CheckRoutingType checks if a message type has the allowed routing type
func CheckRoutingType(messageType pb.MessageType, routingType nnetpb.RoutingType) error {
	switch routingType {
	case nnetpb.DIRECT:
		if _, ok := pb.AllowedDirectMessageType_name[int32(messageType)]; ok {
			return nil
		}
	case nnetpb.RELAY:
		if _, ok := pb.AllowedRelayMessageType_name[int32(messageType)]; ok {
			return nil
		}
	case nnetpb.BROADCAST_PUSH:
		if _, ok := pb.AllowedBroadcastPushMessageType_name[int32(messageType)]; ok {
			return nil
		}
	case nnetpb.BROADCAST_PULL:
		if _, ok := pb.AllowedBroadcastPullMessageType_name[int32(messageType)]; ok {
			return nil
		}
	case nnetpb.BROADCAST_TREE:
		if _, ok := pb.AllowedBroadcastTreeMessageType_name[int32(messageType)]; ok {
			return nil
		}
	}
	return fmt.Errorf("message of type %s is not allowed for routing type %s", messageType.String(), routingType.String())
}
