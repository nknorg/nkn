package node

import (
	"sync"

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
