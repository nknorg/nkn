// ln: abbr. of local node
package node

import (
	"encoding/hex"
	"encoding/json"
	pbnode "github.com/nknorg/nnet/protobuf/node"
	"net/url"
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util"
	"google.golang.org/protobuf/encoding/protojson"
)

type Node struct {
	*pbnode.Node
	*pb.NodeData
	publicKey []byte
	StartTime time.Time

	sync.RWMutex
	syncState           pb.SyncState
	minVerifiableHeight uint32
}

func (n *Node) MarshalJSON() ([]byte, error) {
	var out map[string]interface{}

	s, err := protojson.Marshal(n.Node)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(s, &out)
	if err != nil {
		return nil, err
	}

	delete(out, "data")
	out["id"] = hex.EncodeToString(n.Node.Id)

	s, err = protojson.Marshal(n.NodeData)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(s, &out)
	if err != nil {
		return nil, err
	}

	out["publicKey"] = hex.EncodeToString(n.GetPublicKey())
	out["syncState"] = n.GetSyncState().String()

	return json.Marshal(out)
}

func NewNode(nnetNode *pbnode.Node, nodeData *pb.NodeData) (*Node, error) {
	err := crypto.CheckPublicKey(nodeData.PublicKey)
	if err != nil {
		return nil, err
	}

	node := &Node{
		Node:      nnetNode,
		NodeData:  nodeData,
		publicKey: nodeData.PublicKey,
		StartTime: time.Now(),
		syncState: pb.SyncState_WAIT_FOR_SYNCING,
	}

	return node, nil
}

func (n *Node) GetChordID() []byte {
	return n.Id
}

func (n *Node) GetID() string {
	return util.ChordIDToNodeID(n.GetChordID())
}

func (n *Node) GetPubKey() []byte {
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
