package node

import (
	"encoding/json"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/pb"
	nnetnode "github.com/nknorg/nnet/node"
)

type RemoteNode struct {
	*Node
	localNode *LocalNode
	nnetNode  *nnetnode.RemoteNode

	sync.RWMutex
	height uint32
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
	out["isOutBound"] = remoteNode.nnetNode.IsOutbound

	return json.Marshal(out)
}

func NewRemoteNode(localNode *LocalNode, nnetNode *nnetnode.RemoteNode) (*RemoteNode, error) {
	var nodeData pb.NodeData
	err := proto.Unmarshal(nnetNode.Node.Data, &nodeData)
	if err != nil {
		return nil, err
	}

	node, err := NewNode(nnetNode.Node.Node, &nodeData)
	if err != nil {
		return nil, err
	}

	remoteNode := &RemoteNode{
		Node:      node,
		localNode: localNode,
		nnetNode:  nnetNode,
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

func (remoteNode *RemoteNode) CloseConn() {
	remoteNode.nnetNode.Stop(nil)
}
