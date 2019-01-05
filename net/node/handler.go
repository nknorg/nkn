package node

import (
	"github.com/nknorg/nkn/net/protocol"
	"github.com/nknorg/nkn/pb"
)

// RemoteMessage is the message received from remote nodes
type RemoteMessage struct {
	Sender  protocol.Noder
	Message []byte
}

// Handler handles a message and returns reply, if it should be passed through
// to other message handler and error
type Handler func(msg *RemoteMessage) (reply []byte, shouldCallNext bool, err error)

// handlerStore is the map from message type to message handler
type handlerStore map[pb.MessageType][]Handler

func newHandlerStore() *handlerStore {
	hs := make(handlerStore)
	return &hs
}

// AddHandler adds a message handler to a message type
func (handlerStore handlerStore) AddHandler(messageType pb.MessageType, handler Handler) {
	handlers, ok := handlerStore[messageType]
	if !ok {
		handlerStore[messageType] = make([]Handler, 0)
		handlers = handlerStore[messageType]
	}

	handlerStore[messageType] = append(handlers, handler)
}

// GetHandlers gets all handlers of a message type
func (handlerStore handlerStore) GetHandlers(messageType pb.MessageType) []Handler {
	return handlerStore[messageType]
}
