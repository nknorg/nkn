package messagebuffer

import (
	"encoding/hex"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/nknorg/nkn/v2/util"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/stretchr/testify/require"
)

// to create random MaxHoldingSeconds
func addRandMsg(mb *MessageBuffer, clientId []byte) (pbMsg *pb.Relay, mNode *msgNode) {

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	n := r.Intn(20)
	if n <= 0 {
		n = 8
	}
	src := util.RandomBytes(n)

	n = r.Intn(20)
	if n <= 0 {
		n = 8
	}
	destId := util.RandomBytes(n)

	n = r.Intn(1024)
	if n <= 0 {
		n = 256
	}
	payload := util.RandomBytes(n)

	pbMsg = &pb.Relay{SrcIdentifier: hex.EncodeToString(src), DestId: destId, Payload: payload}

	pbMsg.MaxHoldingSeconds = uint32(r.Intn(20))
	if pbMsg.MaxHoldingSeconds == 0 { // if 0, no cache.
		pbMsg.MaxHoldingSeconds = 1
	}

	mNode = mb.AddMessage(clientId, pbMsg)

	return pbMsg, mNode
}

// go test -v -run=TestNewMessageBuffer
func TestNewMessageBuffer(t *testing.T) {
	config.Parameters.ClientMsgCacheSize = 32 * 1024 * 1024 // cache size

	mb := NewMessageBuffer(true)
	require.NotEqual(t, nil, mb)
	require.NotEqual(t, nil, mb.treeCreate)
	require.NotEqual(t, nil, mb.treeCreate)
	require.NotEqual(t, nil, mb.buffer)
}

// go test -v -run=TestAddMessage
func TestAddMessage(t *testing.T) {
	mb := NewMessageBuffer(false) // don't start buffer monitor routine

	clientId := util.RandomBytes(8)

	require.Equal(t, 0, mb.GetClientMsgSize(clientId))

	pbMsg := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: 0} // cache 0 seconds
	mNode := mb.AddMessage(clientId, pbMsg)
	require.Equal(t, (*msgNode)(nil), mNode)
	require.Equal(t, 0, mb.GetClientMsgSize(clientId))

	pbMsg, mNode = addRandMsg(mb, clientId)

	require.Equal(t, 1, mb.GetClientMsgSize(clientId))
	require.Equal(t, 1, mb.treeCreate.Size())
	require.Equal(t, 1, mb.treeExpire.Size())
	require.Equal(t, mNode.GetSize(), mb.totalBufSize)

	msgs := mb.PopMessages(clientId)
	require.Equal(t, 1, len(msgs))
	require.Equal(t, pbMsg, msgs[0])

	const msgNum = 1000
	var totalSize = 0
	pbMsg1, mNode1 := addRandMsg(mb, clientId) // first msg
	totalSize += mNode1.GetSize()
	for i := 0; i < msgNum; i++ {
		_, mNode := addRandMsg(mb, clientId)
		totalSize += mNode.GetSize()
	}
	pbMsg2 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: math.MaxUint32} // cache MaxUint32 seconds
	mNode2 := mb.AddMessage(clientId, pbMsg2) // last msg

	totalSize += mNode2.GetSize()

	require.Equal(t, msgNum+2, mb.GetClientMsgSize(clientId))
	require.Equal(t, msgNum+2, mb.treeCreate.Size())
	require.Equal(t, msgNum+2, mb.treeExpire.Size())
	require.Equal(t, totalSize, mb.totalBufSize)

	msgs = mb.PopMessages(clientId) // pop out to check what we added in.
	require.Equal(t, msgNum+2, len(msgs))
	require.Equal(t, pbMsg1, msgs[0])        // first msg
	require.Equal(t, pbMsg2, msgs[msgNum+1]) // last msg

	require.Equal(t, 0, mb.GetClientMsgSize(clientId))
	require.Equal(t, 0, mb.treeCreate.Size())
	require.Equal(t, 0, mb.treeExpire.Size())
	require.Equal(t, 0, mb.totalBufSize)

}

// go test -v -run=TestPopMessages
func TestPopMessages(t *testing.T) {
	mb := NewMessageBuffer(false)

	clientId1 := util.RandomBytes(8)
	clientId2 := util.RandomBytes(8)

	const msgNum1 = 100
	pbMsg1, _ := addRandMsg(mb, clientId1)
	for i := 0; i < msgNum1; i++ {
		addRandMsg(mb, clientId1)
	}
	pbMsg2, _ := addRandMsg(mb, clientId1)

	require.Equal(t, msgNum1+2, mb.GetClientMsgSize(clientId1))

	const msgNum2 = 200
	for i := 0; i < msgNum2; i++ {
		addRandMsg(mb, clientId2)
	}

	require.Equal(t, msgNum2, mb.GetClientMsgSize(clientId2))
	// index trees' size
	require.Equal(t, msgNum1+2+msgNum2, mb.treeCreate.Size())
	require.Equal(t, msgNum1+2+msgNum2, mb.treeExpire.Size())

	msgs := mb.PopMessages(clientId1)

	require.Equal(t, msgNum1+2, len(msgs))
	require.Equal(t, 0, mb.GetClientMsgSize(clientId1))
	require.Equal(t, pbMsg1, msgs[0])
	require.Equal(t, pbMsg2, msgs[msgNum1+1])

	require.Equal(t, msgNum2, mb.GetClientMsgSize(clientId2))
	// index trees' size
	require.Equal(t, msgNum2, mb.treeCreate.Size())
	require.Equal(t, msgNum2, mb.treeExpire.Size())

	msgs = mb.PopMessages(clientId2)
	require.Equal(t, msgNum2, len(msgs))
	require.Equal(t, 0, mb.GetClientMsgSize(clientId2))
	// index trees' size
	require.Equal(t, 0, mb.treeCreate.Size())
	require.Equal(t, 0, mb.treeExpire.Size())

}

// go test -v -run=TestRemoveOldest
func TestRemoveOldest(t *testing.T) {
	mb := NewMessageBuffer(false)
	clientId := util.RandomBytes(8)

	const numMsg = 1000
	pbMsgs := make([]*pb.Relay, 0, numMsg)
	for i := 0; i < numMsg; i++ {
		pbMsg, _ := addRandMsg(mb, clientId)
		pbMsgs = append(pbMsgs, pbMsg)
	}

	require.Equal(t, numMsg, mb.treeCreate.Size())
	require.Equal(t, numMsg, mb.treeExpire.Size())
	require.Equal(t, numMsg, mb.GetClientMsgSize(clientId))

	for i := 0; i < numMsg; i++ {
		// remove oldest
		mNode := mb.removeOldest()
		require.Equal(t, pbMsgs[i], mNode.msg) // should remove out at the same order as we added

		require.Equal(t, numMsg-i-1, mb.treeCreate.Size())
		require.Equal(t, numMsg-i-1, mb.treeExpire.Size())
		require.Equal(t, numMsg-i-1, mb.GetClientMsgSize(clientId))
	}

	require.Equal(t, 0, mb.treeCreate.Size())
	require.Equal(t, 0, mb.treeExpire.Size())
	require.Equal(t, 0, mb.GetClientMsgSize(clientId))

}

// go test -v -run=TestRemoveExpired
func TestRemoveExpired(t *testing.T) {
	mb := NewMessageBuffer(false)
	clientId := util.RandomBytes(8)

	m1 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: 3}
	mb.AddMessage(clientId, m1)

	m2 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: 18}
	mb.AddMessage(clientId, m2)

	m3 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: 10}
	mb.AddMessage(clientId, m3)

	require.Equal(t, 3, mb.treeCreate.Size())
	require.Equal(t, 3, mb.treeExpire.Size())
	require.Equal(t, 3, mb.GetClientMsgSize(clientId))

	time.Sleep(5 * time.Second) // sleep 5 seconds, then 1st msg should be removed.

	n, expiredNodes := mb.removeExpired()

	require.Equal(t, 1, n)
	require.Equal(t, m1, expiredNodes[0].msg) // m1 expired

	time.Sleep(6 * time.Second) // sleep 6 seconds, then 3rd msg should be removed.
	n, expiredNodes = mb.removeExpired()
	require.Equal(t, 1, n)
	require.Equal(t, m3, expiredNodes[0].msg) // m3 expired

	time.Sleep(8 * time.Second) // sleep 8 seconds, then 2nd msg should be removed.
	n, expiredNodes = mb.removeExpired()
	require.Equal(t, 1, n)
	require.Equal(t, m2, expiredNodes[0].msg) // m2 expired

}

// go test -v -run=TestClearAll
func TestClearAll(t *testing.T) {
	mb := NewMessageBuffer(false)
	clientId := util.RandomBytes(8)

	const msgNum = 1000
	for i := 0; i < msgNum; i++ {
		addRandMsg(mb, clientId)
	}
	require.Equal(t, msgNum, mb.GetClientMsgSize(clientId))

	mb.ClearAll()
	require.Equal(t, 0, mb.GetClientMsgSize(clientId))
}

// go test -v -run=TestBufferMonitor
func TestBufferMonitor(t *testing.T) {
	mb := NewMessageBuffer(true)
	config.Parameters.ClientMsgCacheSize = 32 * 1024 * 1024 // cache size

	clientId := util.RandomBytes(8)

	m1 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: MonitorInterval - 7} // cache MonitorInterval-7 seconds
	mb.AddMessage(clientId, m1)

	m2 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: []byte("payload"),
		MaxHoldingSeconds: MonitorInterval + 5} // cache MonitorInterval + 5 seconds
	mb.AddMessage(clientId, m2)

	time.Sleep((MonitorInterval + 1) * time.Second) // wait BufferMonitor to execute

	require.Equal(t, 1, mb.treeCreate.Size())
	require.Equal(t, 1, mb.treeExpire.Size())
	require.Equal(t, 1, mb.GetClientMsgSize(clientId))

	time.Sleep((MonitorInterval + 1) * time.Second) // wait BufferMonitor to execute

	require.Equal(t, 0, mb.treeCreate.Size())
	require.Equal(t, 0, mb.treeExpire.Size())
	require.Equal(t, 0, mb.GetClientMsgSize(clientId))

	config.Parameters.ClientMsgCacheSize = 2000 // cache size
	payload := util.RandomBytes(1024)
	m3 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: payload,
		MaxHoldingSeconds: 15} //
	mNode3 := mb.AddMessage(clientId, m3)

	payload = util.RandomBytes(1024)
	m4 := &pb.Relay{SrcIdentifier: "from", DestId: []byte("to"), Payload: payload,
		MaxHoldingSeconds: 15} //
	mNode4 := mb.AddMessage(clientId, m4)

	require.Equal(t, 2, mb.treeCreate.Size())
	require.Equal(t, 2, mb.treeExpire.Size())
	require.Equal(t, 2, mb.GetClientMsgSize(clientId))
	require.Equal(t, mNode3.GetSize()+mNode4.GetSize(), mb.totalBufSize)

	time.Sleep(11 * time.Second) // wait buffer monito to remove oldest

	require.Equal(t, 1, mb.treeCreate.Size())
	require.Equal(t, 1, mb.treeExpire.Size())
	require.Equal(t, 1, mb.GetClientMsgSize(clientId))
	require.Equal(t, mNode4.GetSize(), mb.totalBufSize)

}

// go test -v -run=TestPrintTree
func TestPrintTree(t *testing.T) {
	mb := NewMessageBuffer(false) // don't start buffer monitor routine

	clientId := util.RandomBytes(8)

	const msgNum = 10
	for i := 0; i < msgNum; i++ {
		addRandMsg(mb, clientId)
	}
	printTree(os.Stdout, mb.treeCreate.Root, 2, 'M')

}

func BenchmarkAddMessage(b *testing.B) {
	mb := NewMessageBuffer(false)
	clientId := util.RandomBytes(8)

	for i := 0; i < b.N; i++ {
		addRandMsg(mb, clientId)
	}
	mb.ClearAll()
}

func BenchmarkRemoveOldest(b *testing.B) {
	mb := NewMessageBuffer(false)
	clientId := util.RandomBytes(8)

	for i := 0; i < b.N; i++ {
		addRandMsg(mb, clientId)
	}
	for i := 0; i < b.N; i++ {
		mb.removeOldest()
	}
}
