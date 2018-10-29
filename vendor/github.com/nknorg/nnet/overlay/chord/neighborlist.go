package chord

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/nknorg/nnet/log"
	"github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/protobuf"
)

// NeighborList is a list of nodes with minimal key that is greater than key
type NeighborList struct {
	startID         []byte
	endID           []byte
	reversed        bool
	nodeIDBits      uint32
	maxNumNodes     uint32
	maxNumNodesLock sync.RWMutex
	nodes           sync.Map
}

// NewNeighborList creates a NeighborList
func NewNeighborList(startID, endID []byte, nodeIDBits, maxNumNodes uint32, reversed bool) (*NeighborList, error) {
	if uint32(len(startID)) > nodeIDBits || uint32(len(endID)) > nodeIDBits {
		return nil, fmt.Errorf("key has more than %d bits", nodeIDBits)
	}

	sl := &NeighborList{
		startID:     startID,
		endID:       endID,
		nodeIDBits:  nodeIDBits,
		maxNumNodes: maxNumNodes,
		reversed:    reversed,
	}
	return sl, nil
}

func (sl *NeighborList) cmp(node1, node2 *protobuf.Node) int {
	res := distance(sl.startID, node1.Id, sl.nodeIDBits).Cmp(distance(sl.startID, node2.Id, sl.nodeIDBits))
	if sl.reversed {
		return -res
	}
	return res
}

// IsIDInRange returns if id is in the range of NeighborList
func (sl *NeighborList) IsIDInRange(id []byte) bool {
	if sl.reversed {
		return betweenIncl(sl.endID, sl.startID, id)
	}
	return betweenIncl(sl.startID, sl.endID, id)
}

// IsEmpty returns if there is at least one remote node in NeighborList
func (sl *NeighborList) IsEmpty() bool {
	isEmpty := true
	sl.nodes.Range(func(key, value interface{}) bool {
		isEmpty = false
		return false
	})
	return isEmpty
}

// Len returns the number of remote nodes stored in NeighborList
func (sl *NeighborList) Len() uint32 {
	var length uint32
	sl.nodes.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	return length
}

// Cap returns the maximal number of remote nodes that it can store
func (sl *NeighborList) Cap() uint32 {
	sl.maxNumNodesLock.RLock()
	defer sl.maxNumNodesLock.RUnlock()
	return sl.maxNumNodes
}

// SetMaxNumNodes changes the maximal number of remote nodes that it can store
func (sl *NeighborList) SetMaxNumNodes(maxNumNodes uint32) {
	sl.maxNumNodesLock.Lock()
	sl.maxNumNodes = maxNumNodes
	sl.maxNumNodesLock.Unlock()
}

// GetByID returns the remote node in NeighborList with give id, or nil if no
// such node exists
func (sl *NeighborList) GetByID(id []byte) *node.RemoteNode {
	value, ok := sl.nodes.Load(string(id))
	if ok {
		rn, ok := value.(*node.RemoteNode)
		if ok {
			return rn
		}
	}
	return nil
}

// Exists returns if an id is in NeighborList
func (sl *NeighborList) Exists(id []byte) bool {
	rn := sl.GetByID(id)
	return rn != nil
}

// GetIndex returns the index of the node with an id when NeighborList is
// sorted. Returns -1 if id not found.
func (sl *NeighborList) GetIndex(id []byte) int {
	for i, n := range sl.ToRemoteNodeList(true) {
		if CompareID(n.Id, id) == 0 {
			return i
		}
	}
	return -1
}

// GetFirst returns the remote node in NeighborList that has the smallest
// distance from/to its startID. This is faster than calling ToRemoteNodeList
// and then take the first element
func (sl *NeighborList) GetFirst() *node.RemoteNode {
	var first *node.RemoteNode
	var dist, minDist *big.Int

	sl.nodes.Range(func(key, value interface{}) bool {
		remoteNode, ok := value.(*node.RemoteNode)
		if ok {
			if sl.reversed {
				dist = distance(remoteNode.Id, sl.startID, sl.nodeIDBits)
			} else {
				dist = distance(sl.startID, remoteNode.Id, sl.nodeIDBits)
			}
			if minDist == nil || dist.Cmp(minDist) < 0 {
				first = remoteNode
				minDist = dist
			}
		}
		return true
	})

	return first
}

// GetLast returns the remote node in NeighborList that has the greatest
// distance from/to its startID. This is faster than calling ToRemoteNodeList
// and then take the last element
func (sl *NeighborList) GetLast() *node.RemoteNode {
	var last *node.RemoteNode
	var dist, maxDist *big.Int

	sl.nodes.Range(func(key, value interface{}) bool {
		remoteNode, ok := value.(*node.RemoteNode)
		if ok {
			if sl.reversed {
				dist = distance(remoteNode.Id, sl.startID, sl.nodeIDBits)
			} else {
				dist = distance(sl.startID, remoteNode.Id, sl.nodeIDBits)
			}
			if maxDist == nil || dist.Cmp(maxDist) > 0 {
				last = remoteNode
				maxDist = dist
			}
		}
		return true
	})

	return last
}

// AddOrReplace add a node to NeighborList, replace an existing one if there are
// more than maxNumNodes nodes in list. Returns if node is added, the node that
// is replaced or nil if not, and error if any
func (sl *NeighborList) AddOrReplace(remoteNode *node.RemoteNode) (bool, *node.RemoteNode, error) {
	if remoteNode.IsStopped() || !sl.IsIDInRange(remoteNode.Id) {
		return false, nil, nil
	}

	var replaced *node.RemoteNode

	if sl.Cap() > 0 && sl.Len() >= sl.Cap() {
		replaced = sl.GetLast()
		if replaced != nil && sl.cmp(remoteNode.Node.Node, replaced.Node.Node) >= 0 {
			return false, nil, nil
		}
	}

	_, loaded := sl.nodes.LoadOrStore(string(remoteNode.Id), remoteNode)
	if loaded {
		log.Info("Node already in neighbor list")
		return false, nil, nil
	}

	if replaced != nil {
		sl.Remove(replaced)
	}

	return true, replaced, nil
}

// Remove remove a node from NeighborList, returns if node is in NeighborList
func (sl *NeighborList) Remove(remoteNode *node.RemoteNode) bool {
	if sl.Exists(remoteNode.Id) {
		sl.nodes.Delete(string(remoteNode.Id))
		return true
	}
	return false
}

// ToRemoteNodeList returns a list of RemoteNode that are in NeighborList
func (sl *NeighborList) ToRemoteNodeList(sorted bool) []*node.RemoteNode {
	nodes := make([]*node.RemoteNode, 0)
	sl.nodes.Range(func(key, value interface{}) bool {
		rn, ok := value.(*node.RemoteNode)
		if ok {
			nodes = append(nodes, rn)
		}
		return true
	})

	if sorted {
		sort.Slice(nodes, func(i, j int) bool {
			return sl.cmp(nodes[i].Node.Node, nodes[j].Node.Node) < 0
		})
	}

	return nodes
}

// ToProtoNodeList returns a list of protobuf.Node that are in NeighborList
func (sl *NeighborList) ToProtoNodeList(sorted bool) []*protobuf.Node {
	nodes := make([]*protobuf.Node, 0)
	for _, remoteNode := range sl.ToRemoteNodeList(sorted) {
		nodes = append(nodes, remoteNode.Node.Node)
	}
	return nodes
}

// getNewNodesToConnect query and connect with potentially new nodes that should
// be added to NeighborList
func (sl *NeighborList) getNewNodesToConnect() ([]*protobuf.Node, error) {
	first := sl.GetFirst()
	if first == nil {
		return nil, errors.New("neighbor list is empty")
	}

	var succs, preds []*protobuf.Node
	var err error
	if sl.reversed {
		succs, preds, err = GetSuccAndPred(first, 1, sl.Cap()-1)
	} else {
		succs, preds, err = GetSuccAndPred(first, sl.Cap()-1, 1)
	}
	if err != nil {
		return nil, err
	}

	allNodes := sl.ToProtoNodeList(false)
	allNodes = append(allNodes, succs...)
	allNodes = append(allNodes, preds...)

	seen := make(map[string]struct{}, len(allNodes))
	uniqueNodes := make([]*protobuf.Node, 0)

	for _, n := range allNodes {
		if n == nil || n.Id == nil {
			continue
		}
		if _, ok := seen[string(n.Id)]; ok {
			continue
		}
		seen[string(n.Id)] = struct{}{}
		uniqueNodes = append(uniqueNodes, n)
	}

	sort.Slice(uniqueNodes, func(i, j int) bool {
		return sl.cmp(uniqueNodes[i], uniqueNodes[j]) < 0
	})

	nodesToConnect := make([]*protobuf.Node, 0)

	for i, n := range uniqueNodes {
		if uint32(i) < sl.Cap() && sl.IsIDInRange(n.Id) && !sl.Exists(n.Id) {
			nodesToConnect = append(nodesToConnect, n)
		}
	}

	return nodesToConnect, nil
}
