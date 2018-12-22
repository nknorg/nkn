package message

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

// HandlerStore is the map from message type to message handler
type HandlerStore map[pb.MessageType][]Handler

var handlerStore HandlerStore = make(map[pb.MessageType][]Handler)

// AddHandler adds a message handler to a message type
func AddHandler(messageType pb.MessageType, handler Handler) {
	handlers, ok := handlerStore[messageType]
	if !ok {
		handlerStore[messageType] = make([]Handler, 0)
		handlers = handlerStore[messageType]
	}

	handlerStore[messageType] = append(handlers, handler)
}

// GetHandlers gets all handlers of a message type
func GetHandlers(messageType pb.MessageType) []Handler {
	return handlerStore[messageType]
}
