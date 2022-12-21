package messagebuffer

import (
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
)

const maxSeqId int64 = 1000000 // should not be greater than 1 million, maximum 1 million msg per milli-second
const MonitorInterval = 10     // seconds

// msg node with create time and expire time
type msgNode struct {
	msg      *pb.Relay
	createId int64
	expireId int64
}

func (m *msgNode) GetSize() int {
	// m.createId(int64), m.expireId(int64), m.msg.MaxHoldingSeconds(int32), m.msg.SigChainLen(int32)
	return (8 + 8 + 4 + 4 + len(m.msg.SrcIdentifier) + len(m.msg.SrcPubkey) + len(m.msg.DestId) +
		len(m.msg.Payload) + len(m.msg.BlockHash) + len(m.msg.LastHash))
}

type indexNode struct {
	clientID string
	createId int64
	expireId int64
}

// MessageBuffer is the buffer to hold message for clients not online
// here use several locks, must use locks in the same order to avoid dead lock.
// the order should be: msgMu -> createMu -> expireMu
type MessageBuffer struct {
	sync.Mutex
	buffer     map[string]*rbt.Tree // map ClinetId to msgs tree
	treeCreate *rbt.Tree            // index tree of create time
	treeExpire *rbt.Tree            // index tree of expire time

	totalBufSize    int   // the buffer size in bytes
	lastMilliSecond int64 // last add message time
	seqId           int64 // sequence id
}

// comparation function of red and black tree
func compare(a, b interface{}) (res int) {
	aInt := a.(int64)
	bInt := b.(int64)
	switch {
	case aInt < bInt:
		res = -1
	case aInt == bInt:
		res = 0
	case aInt > bInt:
		res = 1
	}
	return
}

// NewMessageBuffer creates a MessageBuffer
func NewMessageBuffer(startBufferMonitor bool) *MessageBuffer {

	msgBuffer := &MessageBuffer{
		buffer:     make(map[string]*rbt.Tree), // *pb.Relay
		treeCreate: rbt.NewWith(compare),
		treeExpire: rbt.NewWith(compare),
	}

	if startBufferMonitor {
		go msgBuffer.BufferMonitor()
	}

	return msgBuffer
}

// AddMessage adds a message to message buffer
func (msgBuf *MessageBuffer) AddMessage(clientID []byte, msg *pb.Relay) *msgNode {

	if msg.MaxHoldingSeconds <= 0 { // no need cache
		return nil
	}

	clientIDStr := hex.EncodeToString(clientID)
	now := time.Now().UnixMilli()

	msgBuf.Lock()
	defer msgBuf.Unlock()

	if msgBuf.lastMilliSecond != now { // if different milli-second, reset seqId
		msgBuf.lastMilliSecond = now
		msgBuf.seqId = 0
	} else if msgBuf.seqId >= maxSeqId { // if there are more than 1 million messages in 1 ms, reset seqId
		msgBuf.seqId = 0
	}
	msgBuf.seqId++

	createId := now*maxSeqId + msgBuf.seqId
	expireId := (now+(int64(msg.MaxHoldingSeconds)*1000))*maxSeqId + msgBuf.seqId

	mNode := &msgNode{
		msg:      msg,
		createId: createId,
		expireId: expireId,
	}

	_, ok := msgBuf.buffer[clientIDStr]
	if !ok {
		msgBuf.buffer[clientIDStr] = rbt.NewWith(compare)
	}
	msgBuf.buffer[clientIDStr].Put(mNode.createId, mNode)
	msgBuf.totalBufSize += mNode.GetSize()

	// add it to index tree
	idxNode := &indexNode{clientID: clientIDStr, expireId: expireId, createId: createId}
	msgBuf.treeCreate.Put(createId, idxNode)
	msgBuf.treeExpire.Put(expireId, idxNode)

	return mNode
}

// get one clientId's msg size
func (msgBuf *MessageBuffer) GetClientMsgSize(clientID []byte) int {
	treeMsg, ok := msgBuf.buffer[hex.EncodeToString(clientID)]
	if !ok {
		return 0
	}
	return treeMsg.Size()
}

// PopMessages reads and clears all messages of a client
func (msgBuf *MessageBuffer) PopMessages(clientID []byte) []*pb.Relay {
	clientIDStr := hex.EncodeToString(clientID)

	msgBuf.Lock()
	defer msgBuf.Unlock()

	treeMsg, ok := msgBuf.buffer[clientIDStr]
	if !ok {
		return nil
	}

	relayMessage := make([]*pb.Relay, 0, treeMsg.Size())
	for node := treeMsg.Left(); node != nil; node = treeMsg.Left() { // delete them from index tree

		mNode := node.Value.(*msgNode)
		relayMessage = append(relayMessage, mNode.msg)
		treeMsg.Remove(mNode.createId)
		msgBuf.totalBufSize -= mNode.GetSize()

		msgBuf.treeCreate.Remove(mNode.createId)
		msgBuf.treeExpire.Remove(mNode.expireId)

	}

	return relayMessage
}

// remove expired message
func (msgBuf *MessageBuffer) removeExpired() (expiredNum int, expiredNodes []*msgNode) {
	now := time.Now().UnixMilli() + 1 // plus 1 milli-second
	nowId := now * maxSeqId

	expiredNum = 0

	for minNode := msgBuf.treeExpire.Left(); minNode != nil; minNode = msgBuf.treeExpire.Left() {
		idxNode := minNode.Value.(*indexNode)
		if idxNode.expireId < nowId { // expired
			v, ok := msgBuf.buffer[idxNode.clientID].Get(idxNode.createId)
			if ok { // remove from msg buffer
				mNode := v.(*msgNode)
				msgBuf.buffer[idxNode.clientID].Remove(idxNode.createId)
				msgBuf.totalBufSize -= mNode.GetSize()
				expiredNodes = append(expiredNodes, mNode)
			}
			// remove from index tree
			msgBuf.treeExpire.Remove(idxNode.expireId)
			msgBuf.treeCreate.Remove(idxNode.createId)
			expiredNum++
		} else {
			break
		}
	}

	return
}

// remove oldest message
func (msgBuf *MessageBuffer) removeOldest() (removedNode *msgNode) {

	minNode := msgBuf.treeCreate.Left()
	if minNode == nil { // the tree is empty
		return
	}

	createId := minNode.Key.(int64)
	idxNode := minNode.Value.(*indexNode)
	treeMsg, ok := msgBuf.buffer[idxNode.clientID]
	if ok {
		v, ok := treeMsg.Get(createId)
		removedNode = v.(*msgNode)
		if ok {
			treeMsg.Remove(createId)
			mNode := v.(*msgNode)
			msgBuf.totalBufSize -= mNode.GetSize()
		}

		msgBuf.treeExpire.Remove(idxNode.expireId)
		msgBuf.treeCreate.Remove(createId)
	}
	return
}

// clear all cached msg and index
func (msgBuf *MessageBuffer) ClearAll() {
	msgBuf.Lock()
	defer msgBuf.Unlock()

	for _, treeMsg := range msgBuf.buffer {
		treeMsg.Clear()
	}
	msgBuf.treeCreate.Clear()
	msgBuf.treeExpire.Clear()
	msgBuf.totalBufSize = 0
}

// monitor message buffer usage, and remove expired message .
func (msgBuf *MessageBuffer) BufferMonitor() {
	if config.Parameters.ClientMsgCacheSize <= 0 {
		log.Warning("config.Parameters.ClientMsgCacheSize is not set properly. ")
	}

	for {
		timer := time.NewTimer(MonitorInterval * time.Second)

		select {
		case <-timer.C:
		}

		msgBuf.Lock()
		// delete expired msg
		msgBuf.removeExpired()

		// if over size, repeat until totalBufSize less than config.Parameters.ClientMsgCacheSize
		for config.Parameters.ClientMsgCacheSize > 0 && msgBuf.totalBufSize > int(config.Parameters.ClientMsgCacheSize) {
			msgBuf.removeOldest()
		}
		msgBuf.Unlock()

	}
}

// print out tree in indent way.
func printTree(w io.Writer, node *rbt.Node, ns int, ch rune) {
	if node == nil {
		return
	}

	for i := 0; i < ns; i++ {
		fmt.Fprint(w, " ")
	}
	fmt.Fprintf(w, "%c: %v\n", ch, node.Key)
	printTree(w, node.Left, ns+2, 'L')
	printTree(w, node.Right, ns+2, 'R')
}
