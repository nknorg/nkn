package node

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"golang.org/x/crypto/nacl/box"
)

// for encryption
const (
	nonceSize     = 24
	SharedKeySize = 32
)

var EmptyNonce [nonceSize]byte

type RemoteNode struct {
	*Node
	localNode ILocalNode
	NnetNode  *nnetnode.RemoteNode
	sharedKey *[SharedKeySize]byte

	sync.RWMutex
	height         uint32
	lastUpdateTime time.Time
}

func (remoteNode *RemoteNode) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	buf, err := json.Marshal(remoteNode.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, &out)
	if err != nil {
		return nil, err
	}

	out["height"] = remoteNode.GetHeight()
	out["isOutbound"] = remoteNode.NnetNode.IsOutbound
	out["roundTripTime"] = remoteNode.NnetNode.GetRoundTripTime() / time.Millisecond
	out["connTime"] = time.Since(remoteNode.Node.StartTime) / time.Second

	return json.Marshal(out)
}

func NewRemoteNode(localNode ILocalNode, nnetNode *nnetnode.RemoteNode) (*RemoteNode, error) {
	nodeData := &pb.NodeData{}
	err := proto.Unmarshal(nnetNode.Node.Data, nodeData)
	if err != nil {
		return nil, err
	}

	node, err := NewNode(nnetNode.Node.Node, nodeData)
	if err != nil {
		return nil, err
	}

	if len(node.PublicKey) == 0 {
		return nil, errors.New("nil public key")
	}
	sharedKey, err := localNode.ComputeSharedKey(node.PublicKey)
	if err != nil {
		return nil, err
	}

	remoteNode := &RemoteNode{
		Node:      node,
		localNode: localNode,
		NnetNode:  nnetNode,
		sharedKey: sharedKey,
	}

	return remoteNode, nil
}

func (remoteNode *RemoteNode) GetHeight() uint32 {
	remoteNode.RLock()
	defer remoteNode.RUnlock()
	return remoteNode.height
}

func (remoteNode *RemoteNode) SetHeight(height uint32) {
	remoteNode.Lock()
	defer remoteNode.Unlock()
	remoteNode.height = height
}

func (remoteNode *RemoteNode) GetLastUpdateTime() time.Time {
	remoteNode.RLock()
	defer remoteNode.RUnlock()
	return remoteNode.lastUpdateTime
}

func (remoteNode *RemoteNode) SetLastUpdateTime(lastUpdateTime time.Time) {
	remoteNode.Lock()
	defer remoteNode.Unlock()
	remoteNode.lastUpdateTime = lastUpdateTime
}

func (remoteNode *RemoteNode) CloseConn() {
	remoteNode.NnetNode.Stop(nil)
}

func (remoteNode *RemoteNode) String() string {
	return remoteNode.NnetNode.String()
}

func (remoteNode *RemoteNode) IsStopped() bool {
	return remoteNode.NnetNode.IsStopped()
}

func (remoteNode *RemoteNode) SendBytesAsync(buf []byte) error {
	err := remoteNode.localNode.GetNnet().SendBytesDirectAsync(buf, remoteNode.NnetNode)
	if err != nil {
		log.Debugf("Error sending async messge to node %v, removing node.", err.Error())
		remoteNode.CloseConn()
		remoteNode.localNode.RemoveNeighborNode(remoteNode.GetID())
	}
	return err
}

func (remoteNode *RemoteNode) SendBytesSync(buf []byte) ([]byte, error) {
	return remoteNode.SendBytesSyncWithTimeout(buf, 0)
}

func (remoteNode *RemoteNode) SendBytesSyncWithTimeout(buf []byte, replyTimeout time.Duration) ([]byte, error) {
	reply, _, err := remoteNode.localNode.GetNnet().SendBytesDirectSyncWithTimeout(buf, remoteNode.NnetNode, replyTimeout)
	if err != nil {
		log.Debugf("Error sending sync messge to node: %v", err.Error())
	}
	return reply, err
}

func (remoteNode *RemoteNode) SendBytesReply(replyToID, buf []byte) error {
	err := remoteNode.localNode.GetNnet().SendBytesDirectReply(replyToID, buf, remoteNode.NnetNode)
	if err != nil {
		log.Debugf("Error sending async messge to node: %v, removing node.", err.Error())
		remoteNode.CloseConn()
		remoteNode.localNode.RemoveNeighborNode(remoteNode.GetID())
	}
	return err
}

func (remoteNode *RemoteNode) EncryptMessage(message []byte) []byte {
	return EncryptMessage(message, remoteNode.sharedKey)
}

func (remoteNode *RemoteNode) DecryptMessage(message []byte) ([]byte, bool, error) {
	return DecryptMessage(message, remoteNode.sharedKey)
}

func EncryptMessage(message []byte, sharedKey *[SharedKeySize]byte) []byte {
	if sharedKey == nil {
		return append(EmptyNonce[:], message...)
	}

	var nonce [nonceSize]byte
	_, err := rand.Read(nonce[:])
	if err != nil {
		return nil
	}

	encrypted := make([]byte, len(message)+box.Overhead+nonceSize)
	copy(encrypted[:nonceSize], nonce[:])
	box.SealAfterPrecomputation(encrypted[nonceSize:nonceSize], message, &nonce, sharedKey)

	return encrypted
}

func DecryptMessage(message []byte, sharedKey *[SharedKeySize]byte) ([]byte, bool, error) {
	if len(message) < nonceSize {
		return nil, false, fmt.Errorf("encrypted message should have at least %d bytes", nonceSize)
	}

	var nonce [nonceSize]byte
	copy(nonce[:], message[:nonceSize])
	if nonce == EmptyNonce {
		return message[nonceSize:], false, nil
	}

	if sharedKey == nil {
		return nil, false, errors.New("cannot decrypt message: no shared key yet")
	}

	if len(message) < nonceSize+box.Overhead {
		return nil, false, fmt.Errorf("encrypted message should have at least %d bytes", nonceSize+box.Overhead)
	}

	decrypted := make([]byte, len(message)-nonceSize-box.Overhead)
	_, ok := box.OpenAfterPrecomputation(decrypted[:0], message[nonceSize:], &nonce, sharedKey)
	if !ok {
		return nil, false, errors.New("decrypt message failed")
	}

	return decrypted, true, nil
}
