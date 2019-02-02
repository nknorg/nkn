package node

import (
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sync"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/pb"
	nnetpb "github.com/nknorg/nnet/protobuf"
)

type Node struct {
	*nnetpb.Node
	*pb.NodeData
	publicKey *crypto.PubKey

	sync.RWMutex
	syncState pb.SyncState
}

func (node *Node) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	marshaler := &jsonpb.Marshaler{}

	s, err := marshaler.MarshalToString(node.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(s), &out)
	if err != nil {
		return nil, err
	}

	delete(out, "data")
	out["id"] = hex.EncodeToString(node.Node.Id)

	s, err = marshaler.MarshalToString(node.NodeData)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(s), &out)
	if err != nil {
		return nil, err
	}

	out["publicKey"] = hex.EncodeToString(node.NodeData.PublicKey)
	out["syncState"] = node.GetSyncState().String()

	return json.Marshal(out)
}

func NewNode(nnetNode *nnetpb.Node, nodeData *pb.NodeData) (*Node, error) {
	publicKey, err := crypto.DecodePoint(nodeData.PublicKey)
	if err != nil {
		return nil, err
	}

	node := &Node{
		Node:      nnetNode,
		NodeData:  nodeData,
		publicKey: publicKey,
		syncState: pb.WaitForSyncing,
	}

	return node, nil
}

func (node *Node) GetChordID() []byte {
	return node.Id
}

func (node *Node) GetID() string {
	return chordIDToNodeID(node.GetChordID())
}

func (node *Node) GetPubKey() *crypto.PubKey {
	return node.publicKey
}

func (node *Node) GetHostname() string {
	address, _ := url.Parse(node.GetAddr())
	return address.Hostname()
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
