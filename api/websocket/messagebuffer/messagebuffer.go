package messagebuffer

import (
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
)

type Item struct {
	relay        *pb.Relay
	creationTime time.Time
	expiryTime   time.Time
}

type queue struct {
	parent *Buffer
	item   *Item
}

type Buffer struct {
	mux   sync.Mutex
	queue map[string][]*queue
	size  int
}

func New() *Buffer {
	return &Buffer{
		queue: make(map[string][]*queue, 0),
	}
}

func getParent(index int) int {
	if index <= 0 {
		return -1
	}

	return (index - 1) / 2
}

func getChildren(index int) (int, int) {
	tree := index * 2

	return tree + 1, tree + 2
}

func (q *queue) isAfter(node *queue) bool {
	a := q.item
	b := node.item

	if a.creationTime.Before(b.creationTime) {
		return false
	}
	if a.creationTime == b.creationTime {
		return false
	}

	return true
}

func (q *queue) isAfterOrEqual(node *queue) bool {
	a := q.item
	b := node.item

	if a.creationTime.Before(b.creationTime) {
		return false
	}
	if a.creationTime == b.creationTime {
		return true
	}

	return true
}

func (m *Buffer) Purge() {
	var clients []string

	m.mux.Lock()
	for clientIDStr := range m.queue {
		clients = append(clients, clientIDStr)
	}
	m.mux.Unlock()

	for _, clientIDStr := range clients {
		relays := m.PopMessages(clientIDStr)

		for _, relay := range relays {
			m.put(clientIDStr, relay)
		}
	}
}

func (m *Buffer) AddMessage(clientID string, relay *pb.Relay) error {
	if relay.MaxHoldingSeconds <= 0 {
		return nil
	}

	m.Purge()

	if m.size >= config.Parameters.MaxClientMessageBufferSize {
		return fmt.Errorf("Max buffer size reached.")
	}

	return m.put(clientID, relay)
}

func (m *Buffer) PopMessages(clientID string) []*pb.Relay {
	m.mux.Lock()
	defer m.mux.Unlock()

	var messages []*pb.Relay

	s := 0
	item, _ := m.get(clientID)

	for item != nil {
		if item.expiryTime.Before(time.Now()) {
			continue
		}

		messages = append(messages, item.relay)
		s += len(item.relay.Payload)

		item, _ = m.get(clientID)
	}

	m.size -= s

	return messages
}

func (m *Buffer) put(clientID string, relay *pb.Relay) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	q := m.queue[clientID]
	next := len(q)
	now := time.Now()
	node := &queue{
		parent: m,
		item: &Item{
			creationTime: now,
			expiryTime:   now.Add(time.Second * time.Duration(relay.MaxHoldingSeconds)),
			relay:        relay,
		},
	}

	m.queue[clientID] = append(m.queue[clientID], node)

	par := getParent(next)
	cur := next

	for (par >= 0) && (q[par].isAfter(node)) {
		m.queue[clientID][par], m.queue[clientID][cur] = q[cur], q[par]
		cur = par
		par = getParent(cur)
	}

	m.size += len(relay.Payload)

	return nil
}

func (m *Buffer) get(clientID string) (*Item, bool) {
	q := m.queue[clientID]
	nodelen := len(q)

	if nodelen <= 0 {
		return nil, false
	}

	ret := q[0].item

	if nodelen == 1 {
		delete(m.queue, clientID)
		return ret, true
	}

	curlen := nodelen - 1

	m.queue[clientID][0] = q[curlen]
	m.queue[clientID] = q[0:curlen]

	cur := 0
	node := q[0]

	for {
		lchild, rchild := getChildren(cur)

		if lchild >= curlen {
			return ret, true
		}

		if rchild >= curlen {
			if node.isAfter(q[lchild]) {
				m.queue[clientID][cur], m.queue[clientID][lchild] = q[lchild], q[cur]
			}

			return ret, true
		}

		sindex := rchild
		if q[rchild].isAfter(q[lchild]) {
			sindex = lchild
		}

		snode := q[sindex]
		if snode.isAfterOrEqual(node) {
			return ret, true
		}

		m.queue[clientID][cur], m.queue[clientID][sindex] = q[sindex], q[cur]

		cur = sindex
	}
}
