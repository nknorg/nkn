package node

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nnet/cache"
	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/protobuf"
	"github.com/nknorg/nnet/transport"
	"github.com/nknorg/nnet/util"
)

const (
	// Max number of msg received that can be buffered
	remoteRxMsgChanLen = 23333

	// Max number of msg to be sent that can be buffered
	remoteTxMsgChanLen = 23333

	// Timeout for reply message
	replyTimeout = 5 * time.Second

	// Time interval between keep-alive msg
	keepAliveInterval = 3 * time.Second

	// Max idle time before considering node dead and closing connection
	keepAliveTimeout = 10 * time.Second

	// How long a sent message id stays in cache before expiration
	txMsgCacheExpiration = 60 * time.Second

	// How often to check and delete expired sent message
	txMsgCacheCleanupInterval = 10 * time.Second

	// A grace period that allows remote node to send messages in queue
	stopGracePeriod = 100 * time.Millisecond
)

// RemoteNode is a remote node
type RemoteNode struct {
	*Node
	LocalNode  *LocalNode
	IsOutbound bool
	conn       net.Conn
	rxMsgChan  chan *protobuf.Message
	txMsgChan  chan *protobuf.Message
	txMsgCache cache.Cache
	ready      bool
	readyLock  sync.RWMutex
}

// NewRemoteNode creates a remote node
func NewRemoteNode(localNode *LocalNode, conn net.Conn, isOutbound bool) (*RemoteNode, error) {
	if localNode == nil {
		return nil, errors.New("Local node is nil")
	}
	if conn == nil {
		return nil, errors.New("conn is nil")
	}

	node, err := NewNode(nil, "")
	if err != nil {
		return nil, err
	}

	txMsgCache := cache.NewGoCache(txMsgCacheExpiration, txMsgCacheCleanupInterval)

	remoteNode := &RemoteNode{
		Node:       node,
		LocalNode:  localNode,
		conn:       conn,
		IsOutbound: isOutbound,
		rxMsgChan:  make(chan *protobuf.Message, remoteRxMsgChanLen),
		txMsgChan:  make(chan *protobuf.Message, remoteTxMsgChanLen),
		txMsgCache: txMsgCache,
	}

	return remoteNode, nil
}

func (rn *RemoteNode) String() string {
	if !rn.IsReady() {
		return fmt.Sprintf("<%s>", rn.conn.RemoteAddr().String())
	}
	return fmt.Sprintf("%v<%s>", rn.Node, rn.conn.RemoteAddr().String())
}

// IsReady returns if the remote node is ready
func (rn *RemoteNode) IsReady() bool {
	rn.readyLock.RLock()
	defer rn.readyLock.RUnlock()
	return rn.ready
}

// Start starts the runtime loop of the remote node
func (rn *RemoteNode) Start() error {
	rn.StartOnce.Do(func() {
		if rn.IsStopped() {
			return
		}

		go rn.handleMsg()
		go rn.rx()
		go rn.tx()

		go func() {
			n, err := rn.GetNode()
			if err != nil {
				rn.Stop(fmt.Errorf("Get node error: %s", err))
				return
			}

			var existing *RemoteNode
			rn.LocalNode.neighbors.Range(func(key, value interface{}) bool {
				remoteNode, ok := value.(*RemoteNode)
				if ok && remoteNode.IsReady() && bytes.Equal(remoteNode.Id, n.Id) {
					if remoteNode.IsStopped() {
						log.Warningf("Remove stopped remote node %v from list", remoteNode)
						rn.LocalNode.neighbors.Delete(key)
					} else {
						existing = remoteNode
					}
					return false
				}
				return true
			})
			if existing != nil {
				rn.Stop(fmt.Errorf("Node with id %x is already connected at new addr %s", existing.Id, existing.conn.RemoteAddr().String()))
				return
			}

			remoteAddr, err := transport.Parse(n.Addr)
			if err != nil {
				rn.Stop(fmt.Errorf("Parse node addr %s error: %s", n.Addr, err))
				return
			}

			if remoteAddr.Host == "" {
				connAddr := rn.conn.RemoteAddr().String()
				remoteAddr.Host, _, err = net.SplitHostPort(connAddr)
				if err != nil {
					rn.Stop(fmt.Errorf("Parse conn remote addr %s error: %s", connAddr, err))
					return
				}
				n.Addr = remoteAddr.String()
			}

			rn.Node.Node = n

			rn.readyLock.Lock()
			rn.ready = true
			rn.readyLock.Unlock()

			for _, f := range rn.LocalNode.middlewareStore.remoteNodeReady {
				if !f(rn) {
					break
				}
			}
		}()
	})

	return nil
}

// Stop stops the runtime loop of the remote node
func (rn *RemoteNode) Stop(err error) {
	rn.StopOnce.Do(func() {
		if err != nil {
			log.Warningf("Remote node %v stops because of error: %s", rn, err)
		} else {
			log.Infof("Remote node %v stops", rn)
		}

		err = rn.NotifyStop()
		if err != nil {
			log.Warning("Notify remote node stop error:", err)
		}

		time.Sleep(stopGracePeriod)

		rn.LifeCycle.Stop()

		if rn.conn != nil {
			rn.LocalNode.neighbors.Delete(rn.conn.RemoteAddr().String())
			rn.conn.Close()
		}

		for _, f := range rn.LocalNode.middlewareStore.remoteNodeDisconnected {
			if !f(rn) {
				break
			}
		}
	})
}

// handleMsg starts a loop that handles received msg
func (rn *RemoteNode) handleMsg() {
	var msg *protobuf.Message
	var remoteMsg *RemoteMessage
	var msgChan chan *RemoteMessage
	var err error
	keepAliveTimeoutTimer := time.NewTimer(keepAliveTimeout)

	for {
		if rn.IsStopped() {
			util.StopTimer(keepAliveTimeoutTimer)
			return
		}

		select {
		case msg = <-rn.rxMsgChan:
			remoteMsg, err = NewRemoteMessage(rn, msg)
			if err != nil {
				log.Error(err)
				continue
			}

			msgChan, err = rn.LocalNode.GetRxMsgChan(msg.RoutingType)
			if err != nil {
				log.Error(err)
				continue
			}

			select {
			case msgChan <- remoteMsg:
			default:
				log.Warningf("Msg chan full for routing type %d, discarding msg", msg.RoutingType)
			}
		case <-keepAliveTimeoutTimer.C:
			rn.Stop(errors.New("keepalive timeout"))
		}

		util.ResetTimer(keepAliveTimeoutTimer, keepAliveTimeout)
	}
}

// handleMsgBuf unmarshal buf to msg and send it to msg chan of the local node
func (rn *RemoteNode) handleMsgBuf(buf []byte) {
	msg := &protobuf.Message{}
	err := proto.Unmarshal(buf, msg)
	if err != nil {
		rn.Stop(fmt.Errorf("unmarshal msg error: %s", err))
		return
	}

	select {
	case rn.rxMsgChan <- msg:
	default:
		log.Warning("Rx msg chan full, discarding msg")
	}
}

// rx receives and handle data from RemoteNode rn
func (rn *RemoteNode) rx() {
	msgLenBuf := make([]byte, msgLenBytes)
	var readLen int

	for {
		if rn.IsStopped() {
			return
		}

		l, err := rn.conn.Read(msgLenBuf)
		if err != nil {
			rn.Stop(fmt.Errorf("Read msg len error: %s", err))
			continue
		}
		if l != msgLenBytes {
			rn.Stop(fmt.Errorf("Msg len has %d bytes, which is less than expected %d", l, msgLenBytes))
			continue
		}

		msgLen := int(binary.BigEndian.Uint32(msgLenBuf))
		if msgLen < 0 {
			rn.Stop(fmt.Errorf("Msg len %d overflow", msgLen))
			continue
		}

		buf := make([]byte, msgLen)

		for readLen = 0; readLen < msgLen; readLen += l {
			l, err = rn.conn.Read(buf[readLen:])
			if err != nil {
				break
			}
		}

		if err != nil {
			rn.Stop(fmt.Errorf("Read msg error: %s", err))
			continue
		}

		if readLen > msgLen {
			rn.Stop(fmt.Errorf("Msg has %d bytes, which is more than expected %d", readLen, msgLen))
			continue
		}

		rn.handleMsgBuf(buf)
	}
}

// tx marshals and sends data in txMsgChan to RemoteNode rn
func (rn *RemoteNode) tx() {
	var msg *protobuf.Message
	var buf []byte
	var err error
	msgLenBuf := make([]byte, msgLenBytes)
	keepAliveTimer := time.NewTimer(keepAliveInterval)

	for {
		if rn.IsStopped() {
			util.StopTimer(keepAliveTimer)
			return
		}

		select {
		case msg = <-rn.txMsgChan:
			buf, err = proto.Marshal(msg)
			if err != nil {
				log.Error(err)
				continue
			}

			binary.BigEndian.PutUint32(msgLenBuf, uint32(len(buf)))

			_, err = rn.conn.Write(msgLenBuf)
			if err != nil {
				rn.Stop(fmt.Errorf("Write to conn error: %s", err))
				continue
			}

			_, err = rn.conn.Write(buf)
			if err != nil {
				rn.Stop(fmt.Errorf("Write to conn error: %s", err))
				continue
			}
		case <-keepAliveTimer.C:
			rn.keepAlive()
		}

		util.ResetTimer(keepAliveTimer, keepAliveInterval)
	}
}

// SendMessage marshals and sends msg, will returns a RemoteMessage chan if hasReply is true
func (rn *RemoteNode) SendMessage(msg *protobuf.Message, hasReply bool) (<-chan *RemoteMessage, error) {
	if rn.IsStopped() {
		return nil, errors.New("Remote node has stopped")
	}

	if len(msg.MessageId) == 0 {
		return nil, errors.New("Message ID is empty")
	}

	_, found := rn.txMsgCache.Get(msg.MessageId)
	if found {
		return nil, nil
	}

	err := rn.txMsgCache.Add(msg.MessageId, struct{}{})
	if err != nil {
		return nil, err
	}

	select {
	case rn.txMsgChan <- msg:
	default:
		return nil, errors.New("Tx msg chan full, discarding msg")
	}

	if hasReply {
		return rn.LocalNode.AllocReplyChan(msg.MessageId)
	}

	return nil, nil
}

// SendMessageAsync sends msg and returns if there is an error
func (rn *RemoteNode) SendMessageAsync(msg *protobuf.Message) error {
	_, err := rn.SendMessage(msg, false)
	return err
}

// SendMessageSync sends msg, returns reply message or error if timeout
func (rn *RemoteNode) SendMessageSync(msg *protobuf.Message) (*RemoteMessage, error) {
	replyChan, err := rn.SendMessage(msg, true)
	if err != nil {
		return nil, err
	}

	select {
	case replyMsg := <-replyChan:
		return replyMsg, nil
	case <-time.After(replyTimeout):
		return nil, errors.New("Wait for reply timeout")
	}
}

func (rn *RemoteNode) keepAlive() error {
	msg, err := NewPingMessage()
	if err != nil {
		return err
	}

	err = rn.SendMessageAsync(msg)
	if err != nil {
		return err
	}

	return nil
}

// Ping sends a Ping message to remote node and wait for reply
func (rn *RemoteNode) Ping() error {
	msg, err := NewPingMessage()
	if err != nil {
		return err
	}

	_, err = rn.SendMessageSync(msg)
	if err != nil {
		return err
	}

	return nil
}

// GetNode sends a GetNode message to remote node and wait for reply
func (rn *RemoteNode) GetNode() (*protobuf.Node, error) {
	msg, err := NewGetNodeMessage()
	if err != nil {
		return nil, err
	}

	reply, err := rn.SendMessageSync(msg)
	if err != nil {
		return nil, err
	}

	replyBody := &protobuf.GetNodeReply{}
	err = proto.Unmarshal(reply.Msg.Message, replyBody)
	if err != nil {
		return nil, err
	}

	return replyBody.Node, nil
}

// NotifyStop sends a Stop message to remote node to notify it that we will
// close connection with it
func (rn *RemoteNode) NotifyStop() error {
	msg, err := NewStopMessage()
	if err != nil {
		return err
	}

	err = rn.SendMessageAsync(msg)
	if err != nil {
		return err
	}

	return nil
}
