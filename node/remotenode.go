package node

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/pb"
	nnetnode "github.com/nknorg/nnet/node"
)

type RemoteNode struct {
	*Node
	localNode *LocalNode
	nnetNode  *nnetnode.RemoteNode
	sharedKey *[sharedKeySize]byte

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
	out["isOutbound"] = remoteNode.nnetNode.IsOutbound
	out["roundTripTime"] = remoteNode.nnetNode.GetRoundTripTime() / time.Millisecond

	return json.Marshal(out)
}

func NewRemoteNode(localNode *LocalNode, nnetNode *nnetnode.RemoteNode) (*RemoteNode, error) {
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
	sharedKey, err := localNode.computeSharedKey(node.PublicKey)
	if err != nil {
		return nil, err
	}

	remoteNode := &RemoteNode{
		Node:      node,
		localNode: localNode,
		nnetNode:  nnetNode,
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
	remoteNode.nnetNode.Stop(nil)
}

func (remoteNode *RemoteNode) String() string {
	return remoteNode.nnetNode.String()
}
