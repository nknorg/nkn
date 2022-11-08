package messagebuffer

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/pb"
)

type Buffer struct {
	mux    sync.Mutex
	equeue *ExpirationQueue
	mqueue map[string]*MessageQueue
	size   int
}

func New() *Buffer {
	eq := make(ExpirationQueue, 0)
	heap.Init(&eq)

	return &Buffer{
		equeue: &eq,
		mqueue: make(map[string]*MessageQueue, 0),
	}
}

func (b *Buffer) AddMessage(clientID string, relay *pb.Relay) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if relay.MaxHoldingSeconds <= 0 {
		return nil
	}

	if b.mqueue[clientID] == nil {
		mq := make(MessageQueue, 0)
		heap.Init(&mq)
		b.mqueue[clientID] = &mq
	}

	// if b.size >= config.Parameters.MaxClientMessageBufferSize {
	if b.size >= 1000 {
		b.purge()
	}

	if b.size >= 1000 {
		return fmt.Errorf("Max buffer size reached.")
	}

	now := time.Now()
	expiry := now.Add(time.Second * time.Duration(relay.MaxHoldingSeconds))
	item := &MessageItem{
		relay:        relay,
		creationTime: now,
		expiryTime:   expiry,
		index:        b.mqueue[clientID].Len() - 1,
	}
	heap.Push(b.mqueue[clientID], item)
	heap.Push(b.equeue, &ExpirationItem{
		clientID: clientID,
		index:    b.equeue.Len() - 1,
	})

	b.size += len(item.relay.Payload)

	return nil
}

func (b *Buffer) PopMessages(clientID string) []*pb.Relay {
	b.mux.Lock()
	defer b.mux.Unlock()

	if b.mqueue[clientID] == nil {
		return nil
	}

	var messages []*pb.Relay

	item := b.popMessageItem(clientID)
	s := 0

	for item != nil {
		if item.expiryTime.Before(time.Now()) {
			item = b.popMessageItem(clientID)
			continue
		}

		messages = append(messages, item.relay)
		s += len(item.relay.Payload)

		item = b.popMessageItem(clientID)
	}

	b.size -= s

	return messages
}

func (b *Buffer) popMessageItem(clientID string) *MessageItem {
	if b.mqueue[clientID].Len() == 0 {
		return nil
	}

	item, ok := heap.Pop(b.mqueue[clientID]).(*MessageItem)
	if !ok {
		return nil
	}

	return item
}
func (b *Buffer) popExpirationItem() *ExpirationItem {
	if b.equeue.Len() == 0 {
		return nil
	}

	item, ok := heap.Pop(b.equeue).(*ExpirationItem)
	if !ok {
		return nil
	}

	return item
}

func (b *Buffer) purge() error {
	item := b.popExpirationItem()

	// for item != nil {
	b.popMessageItem(item.clientID)
	item = b.popExpirationItem()
	// }

	return nil
}
