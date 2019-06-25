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
	syncState           pb.SyncState
	minVerifiableHeight uint32
}

func (n *Node) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	marshaler := &jsonpb.Marshaler{}

	s, err := marshaler.MarshalToString(n.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(s), &out)
	if err != nil {
		return nil, err
	}

	delete(out, "data")
	out["id"] = hex.EncodeToString(n.Node.Id)

	s, err = marshaler.MarshalToString(n.NodeData)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(s), &out)
	if err != nil {
		return nil, err
	}

	out["publicKey"] = hex.EncodeToString(n.NodeData.PublicKey)
	out["syncState"] = n.GetSyncState().String()

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
		syncState: pb.WAIT_FOR_SYNCING,
	}

	return node, nil
}

func (n *Node) GetChordID() []byte {
	return n.Id
}

func (n *Node) GetID() string {
	return chordIDToNodeID(n.GetChordID())
}

func (n *Node) GetPubKey() *crypto.PubKey {
	return n.publicKey
}

func (n *Node) GetHostname() string {
	address, _ := url.Parse(n.GetAddr())
	return address.Hostname()
}

func (n *Node) GetSyncState() pb.SyncState {
	n.RLock()
	defer n.RUnlock()
	return n.syncState
}

func (n *Node) SetSyncState(s pb.SyncState) bool {
	n.Lock()
	defer n.Unlock()
	if n.syncState == s {
		return false
	}
	n.syncState = s
	return true
}

func (n *Node) GetMinVerifiableHeight() uint32 {
	n.RLock()
	defer n.RUnlock()
	return n.minVerifiableHeight
}

func (n *Node) SetMinVerifiableHeight(height uint32) {
	n.Lock()
	defer n.Unlock()
	n.minVerifiableHeight = height
}
