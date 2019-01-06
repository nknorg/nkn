package node

import (
	"github.com/nknorg/nkn/pb"
)

// RemoteMessage is the message received from remote nodes
type RemoteMessage struct {
	Sender  *Node
	Message []byte
}

// MessageHandler handles a message and returns reply, if it should be passed
// through to other message handler and error
type MessageHandler func(msg *RemoteMessage) (reply []byte, shouldCallNext bool, err error)

// messageHandlerStore is the map from message type to message handler
type messageHandlerStore map[pb.MessageType][]MessageHandler

func newMessageHandlerStore() *messageHandlerStore {
	hs := make(messageHandlerStore)
	return &hs
}

// AddMessageHandler adds a message handler to a message type
func (handlerStore messageHandlerStore) AddMessageHandler(messageType pb.MessageType, handler MessageHandler) {
	handlers, ok := handlerStore[messageType]
	if !ok {
		handlerStore[messageType] = make([]MessageHandler, 0)
		handlers = handlerStore[messageType]
	}

	handlerStore[messageType] = append(handlers, handler)
}

// GetMessageHandlers gets all handlers of a message type
func (handlerStore messageHandlerStore) GetMessageHandlers(messageType pb.MessageType) []MessageHandler {
	return handlerStore[messageType]
}
