package messagebuffer

import (
	"encoding/hex"
	"sync"

	"github.com/nknorg/nkn/pb"
)

// MessageBuffer is the buffer to hold message for clients not online
type MessageBuffer struct {
	sync.Mutex
	buffer map[string][]*pb.Relay
}

// NewMessageBuffer creates a MessageBuffer
func NewMessageBuffer() *MessageBuffer {
	return &MessageBuffer{
		buffer: make(map[string][]*pb.Relay),
	}
}

// AddMessage adds a message to message buffer
func (messageBuffer *MessageBuffer) AddMessage(clientID []byte, msg *pb.Relay) {
	clientIDStr := hex.EncodeToString(clientID)
	messageBuffer.Lock()
	defer messageBuffer.Unlock()
	messageBuffer.buffer[clientIDStr] = append(messageBuffer.buffer[clientIDStr], msg)
}

// PopMessages reads and clears all messages of a client
func (messageBuffer *MessageBuffer) PopMessages(clientID []byte) []*pb.Relay {
	clientIDStr := hex.EncodeToString(clientID)
	messageBuffer.Lock()
	defer messageBuffer.Unlock()
	messages := messageBuffer.buffer[clientIDStr]
	messageBuffer.buffer[clientIDStr] = nil
	return messages
}
