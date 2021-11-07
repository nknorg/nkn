package messagebuffer

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/pb"
)

// RelayMessage is the node of a bidirectional linked list which is used to store message queue
type RelayMessage struct {
	prev        *RelayMessage
	next        *RelayMessage
	sequenceNum uint64
	message     *pb.Relay
	addedTime   time.Time
}

// MessageQueue is the message queue of a specified client
type MessageQueue struct {
	header *RelayMessage
	tail   *RelayMessage
	length int
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		header: nil,
		tail:   nil,
		length: 0,
	}
}

func (mq *MessageQueue) pop() *RelayMessage {
	// if no message in the queue, return nil
	if mq.header == nil {
		return nil
	}
	// if only one message in the queue, tail pointer should be modified
	if mq.header.next == nil {
		relayMessage := mq.header
		mq.header = nil
		mq.tail = nil
		mq.length--
		return relayMessage

	}
	// if more than one message in the queue, just move forward the header pointer
	relayMessage := mq.header
	mq.header.next.prev = nil
	mq.header = mq.header.next
	relayMessage.next = nil
	mq.length--
	return relayMessage
}

func (mq *MessageQueue) append(message *pb.Relay, sequenceNum uint64) *RelayMessage {
	// if no message in the queue, create a new node
	if mq.tail == nil {
		mq.tail = &RelayMessage{
			prev:        nil,
			next:        nil,
			sequenceNum: sequenceNum,
			message:     message,
			addedTime:   time.Now(),
		}
		mq.header = mq.tail
		mq.length++
		return mq.tail
	}
	// if at least one message in the queue, append the message to the tail
	mq.tail.next = &RelayMessage{
		prev:        mq.tail,
		next:        nil,
		sequenceNum: sequenceNum,
		message:     message,
		addedTime:   time.Now(),
	}
	mq.tail = mq.tail.next
	mq.length++
	return mq.tail
}

// WARNING: make sure the message parameter is the pointer of a element in the queue
func (mq *MessageQueue) delete(message *RelayMessage) {
	// empty queue
	if mq.header == nil {
		return
	}
	mq.length--
	// only one message
	if message == mq.header && message == mq.tail {
		mq.header = nil
		mq.tail = nil
		message = nil
		return
	}
	// more than one message, and the parameter point to header
	if message == mq.header {
		mq.header = message.next
		message.next.prev = nil
		message.next = nil
		message = nil
		return
	}
	// more than one message, and the parameter point to tail
	if message == mq.tail {
		mq.tail = message.prev
		message.prev.next = nil
		message.prev = nil
		message = nil
		return
	}
	// more than one message, and the parameter is NOT header and tail
	message.prev.next = message.next
	message.next.prev = message.prev
	message.prev = nil
	message.next = nil
	message = nil
}

// MessageBuffer is the buffer to hold message for clients not online
type MessageBuffer struct {
	sync.Mutex
	buffer        map[string]*MessageQueue
	abortChannels map[uint64]chan struct{} // used to notify the expiration goroutine
	sequenceNum   uint64                   // sequence number used to indentify specified message
	capacity      int
	length        int
}

// NewMessageBuffer creates a MessageBuffer
func NewMessageBuffer() *MessageBuffer {
	return &MessageBuffer{
		buffer:        make(map[string]*MessageQueue),
		abortChannels: make(map[uint64]chan struct{}),
		sequenceNum:   0,
		capacity:      2000, // TODO: use config file, env variable, command parameter, default value, etc.
		length:        0,
	}
}

func (messageBuffer *MessageBuffer) removeOldestMessage() {
	var oldest *MessageQueue = nil
	var client string = ""
	for id, queue := range messageBuffer.buffer {
		if oldest == nil {
			oldest = queue
			client = id
			continue
		}
		if oldest.header.addedTime.After(queue.header.addedTime) {
			oldest = queue
			client = id
		}
	}
	message := oldest.pop()
	if oldest.length == 0 {
		delete(messageBuffer.buffer, client)
	}
	messageBuffer.length--
	abort := messageBuffer.abortChannels[message.sequenceNum]
	abort <- struct{}{}
}

// AddMessage adds a message to message buffer
func (messageBuffer *MessageBuffer) AddMessage(clientID []byte, msg *pb.Relay) {
	clientIDStr := hex.EncodeToString(clientID)
	messageBuffer.Lock()
	// remove message from cache due to RAM limit
	for {
		if messageBuffer.length >= messageBuffer.capacity {
			messageBuffer.removeOldestMessage()
			continue
		}
		break
	}
	sn := messageBuffer.sequenceNum
	messageQueue, ok := messageBuffer.buffer[clientIDStr]
	if !ok {
		messageBuffer.buffer[clientIDStr] = NewMessageQueue()
		messageQueue = messageBuffer.buffer[clientIDStr]
	}
	relayMessage := messageQueue.append(msg, sn)
	messageBuffer.length++
	messageBuffer.abortChannels[sn] = make(chan struct{}, 1)
	abort := messageBuffer.abortChannels[sn]
	messageBuffer.sequenceNum++
	messageBuffer.Unlock()
	go func() {
		timer := time.After(time.Duration(msg.MaxHoldingSeconds) * time.Second)
		select {
		case <-timer: // expiration
			messageBuffer.Lock()
			messageQueue.delete(relayMessage)
			delete(messageBuffer.abortChannels, sn)
			if messageQueue.length == 0 {
				delete(messageBuffer.buffer, clientIDStr)
			}
			messageBuffer.length--
			messageBuffer.Unlock()
		case <-abort: // consumed or removed
			messageBuffer.Lock()
			delete(messageBuffer.abortChannels, sn)
			messageBuffer.Unlock()
		}
	}()
}

// PopMessages reads and clears all messages of a client
func (messageBuffer *MessageBuffer) PopMessages(clientID []byte) []*pb.Relay {
	clientIDStr := hex.EncodeToString(clientID)
	messageBuffer.Lock()
	defer messageBuffer.Unlock()
	messageQueue := messageBuffer.buffer[clientIDStr]
	messages := make([]*pb.Relay, messageQueue.length)
	len := messageQueue.length
	for i := 0; i < len; i++ {
		relayMessage := messageQueue.pop()
		messageBuffer.length--
		messages[i] = relayMessage.message
		abort := messageBuffer.abortChannels[relayMessage.sequenceNum]
		abort <- struct{}{}
	}
	delete(messageBuffer.buffer, clientIDStr)
	return messages
}
