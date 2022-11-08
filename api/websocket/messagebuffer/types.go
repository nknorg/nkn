package messagebuffer

import (
	"time"

	"github.com/nknorg/nkn/v2/pb"
)

type ExpirationItem struct {
	clientID string
	index    int
}

type ExpirationQueue []*ExpirationItem

func (e ExpirationQueue) Len() int { return len(e) }

func (e ExpirationQueue) Less(i, j int) bool {
	return e[i].index < e[j].index
}

func (e ExpirationQueue) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
	e[i].index = i
	e[j].index = j
}

func (e *ExpirationQueue) Push(x any) {
	n := len(*e)
	item := x.(*ExpirationItem)
	item.index = n
	*e = append(*e, item)
}

func (e *ExpirationQueue) Pop() any {
	old := *e
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*e = old[0 : n-1]
	return item
}

type MessageQueue []*MessageItem

type MessageItem struct {
	relay        *pb.Relay
	creationTime time.Time
	expiryTime   time.Time
	index        int
}

func (m MessageQueue) Len() int { return len(m) }

func (m MessageQueue) Less(i, j int) bool {
	return m[i].creationTime.Before(m[j].creationTime)
}

func (m MessageQueue) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
	m[i].index = i
	m[j].index = j
}

func (m *MessageQueue) Push(x any) {
	n := len(*m)
	item := x.(*MessageItem)
	item.index = n
	*m = append(*m, item)
}

func (m *MessageQueue) Pop() any {
	old := *m
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*m = old[0 : n-1]
	return item
}
