package node

import (
	"sync"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
)

type Node struct {
	sync.RWMutex
	pb.NodeData
	id        uint64
	publicKey *crypto.PubKey
	height    uint32
	syncState pb.SyncState
}

func NewNode(chordID []byte, publicKey *crypto.PubKey, nodeData pb.NodeData) (*Node, error) {
	id, err := chordIDToNodeID(chordID)
	if err != nil {
		return nil, err
	}

	node := &Node{
		NodeData:  nodeData,
		id:        id,
		publicKey: publicKey,
		syncState: pb.WaitForSyncing,
	}

	return node, nil
}

func (node *Node) GetID() uint64 {
	return node.id
}

func (node *Node) GetHttpJsonPort() uint16 {
	return uint16(node.JsonRpcPort)
}

func (node *Node) GetWsPort() uint16 {
	return uint16(node.WebsocketPort)
}

func (node *Node) GetPubKey() *crypto.PubKey {
	return node.publicKey
}

func (node *Node) GetHeight() uint32 {
	node.RLock()
	defer node.RUnlock()
	return node.height
}

func (node *Node) SetHeight(height uint32) {
	node.Lock()
	defer node.Unlock()
	node.height = height
}

func (node *Node) GetSyncState() pb.SyncState {
	node.RLock()
	defer node.RUnlock()
	return node.syncState
}

func (node *Node) SetSyncState(s pb.SyncState) {
	node.Lock()
	defer node.Unlock()
	node.syncState = s
}
