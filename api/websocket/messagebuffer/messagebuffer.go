package messagebuffer

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
)

// MessageBuffer is the buffer to hold message for clients not online
type MessageBuffer struct {
	// ttlcache.Cache[K comparable, V any].
	// we use *pb.Relay as keys here since pointers are comparable.
	c *ttlcache.Cache[*pb.Relay, string]
}

// NewMessageBuffer creates a MessageBuffer
func NewMessageBuffer() *MessageBuffer {
	cache := ttlcache.New(
		ttlcache.WithDisableTouchOnHit[*pb.Relay, string](),
	)

	messageBuffer := &MessageBuffer{
		c: cache,
	}

	return messageBuffer
}

func (m *MessageBuffer) isCacheFull(msg *pb.Relay) bool {
	m.c.DeleteExpired()

	relays := m.c.Keys()
	cachesize := uint32(len(msg.Payload))

	// MaxMessageCacheSize only considers payload.
	for _, relay := range relays {
		cachesize += uint32(len(relay.Payload))
	}

	if cachesize > config.Parameters.MaxMessageCacheSize {
		return true
	}
	return false
}

// AddMessage adds a message to message buffer
func (m *MessageBuffer) AddMessage(clientID []byte, msg *pb.Relay) {
	if m.isCacheFull(msg) {
		return
	}

	m.c.Set(msg, hex.EncodeToString(clientID), time.Second*time.Duration(msg.MaxHoldingSeconds))
}

// PopMessages reads and clears all messages of a client
func (m *MessageBuffer) PopMessages(clientID []byte) []*pb.Relay {
	clientIDStr := hex.EncodeToString(clientID)
	items := m.c.Items()

	var messages []*pb.Relay

	for relay, item := range items {
		if item.Value() == clientIDStr {
			messages = append(messages, relay)
			m.c.Delete(relay)
		}
	}

	return messages
}

func (m *MessageBuffer) Metrics() string {
	metrics := m.c.Metrics()
	return fmt.Sprintf("Insertions: %d - Hits: %d - Misses: %d - Evictions: %d", metrics.Insertions, metrics.Hits, metrics.Misses, metrics.Evictions)
}
