package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/nknorg/nkn/net/protocol"
	nnetnode "github.com/nknorg/nnet/node"
)

// The neighbor node list
type nbrNodes struct {
	List sync.Map
}

func (nm *nbrNodes) GetNbrNode(uid uint64) *RemoteNode {
	v, ok := nm.List.Load(uid)
	if !ok {
		return nil
	}
	n, ok := v.(*RemoteNode)
	if ok {
		return n
	}
	return nil
}

func (nm *nbrNodes) AddNbrNode(remoteNode *RemoteNode) error {
	nm.List.LoadOrStore(remoteNode.GetID(), remoteNode)
	return nil
}

func (nm *nbrNodes) DelNbrNode(id uint64) {
	nm.List.Delete(id)
}

func (nm *nbrNodes) GetConnectionCnt() uint {
	return uint(len(nm.GetNeighbors(nil)))
}

func (nm *nbrNodes) GetNeighborAddrs() ([]protocol.NodeAddr, uint) {
	var addrs []protocol.NodeAddr
	for _, n := range nm.GetNeighbors(nil) {
		ip, _ := n.GetAddr16()
		addrs = append(addrs, protocol.NodeAddr{
			IpAddr:  ip,
			IpStr:   n.GetAddr(),
			InOut:   n.GetConnDirection(),
			Time:    time.Now().UnixNano(),
			Port:    n.GetPort(),
			ID:      n.GetID(),
			NKNaddr: fmt.Sprintf("%x", n.GetChordAddr()),
		})
	}
	return addrs, uint(len(addrs))
}

func (nm *nbrNodes) GetNeighborHeights() ([]uint32, uint) {
	heights := []uint32{}
	for _, n := range nm.GetNeighbors(nil) {
		heights = append(heights, n.GetHeight())
	}
	return heights, uint(len(heights))
}

func (nm *nbrNodes) GetNeighbors(filter func(*RemoteNode) bool) []*RemoteNode {
	neighbors := make([]*RemoteNode, 0)
	nm.List.Range(func(key, value interface{}) bool {
		if rn, ok := value.(*RemoteNode); ok {
			if filter == nil || filter(rn) {
				neighbors = append(neighbors, rn)
			}
		}
		return true
	})
	return neighbors
}

func (localNode *LocalNode) getNbrByNNetNode(nnetRemoteNode *nnetnode.RemoteNode) *RemoteNode {
	if nnetRemoteNode == nil {
		return nil
	}

	nodeID, err := chordIDToNodeID(nnetRemoteNode.Id)
	if err != nil {
		return nil
	}

	nbr := localNode.GetNbrNode(nodeID)
	return nbr
}
