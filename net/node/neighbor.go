package node

import (
	"fmt"
	"sync"

	"github.com/nknorg/nkn/net/protocol"
)

// The neighbor node list
type nbrNodes struct {
	List sync.Map
}

func (nm *nbrNodes) GetNbrNode(uid uint64) *node {
	v, ok := nm.List.Load(uid)
	if !ok {
		return nil
	}
	n, ok := v.(*node)
	if ok {
		return n
	}
	return nil
}

func (nm *nbrNodes) AddNbrNode(n protocol.Noder) {
	node, ok := n.(*node)
	if !ok {
		fmt.Println("Convert the noder error when add node")
		return
	}
	nm.List.LoadOrStore(n.GetID(), node)
}

func (nm *nbrNodes) DelNbrNode(id uint64) (protocol.Noder, bool) {
	v, ok := nm.List.Load(id)
	if !ok {
		return nil, false
	}
	n, _ := v.(*node)
	nm.List.Delete(id)
	return n, true
}

func (nm *nbrNodes) GetConnectionCnt() uint {
	return uint(len(nm.GetNeighborNoder()))
}

func (nm *nbrNodes) GetNeighborAddrs() ([]protocol.NodeAddr, uint) {
	var addrs []protocol.NodeAddr
	for _, n := range nm.GetNeighborNoder() {
		ip, _ := n.GetAddr16()
		addrs = append(addrs, protocol.NodeAddr{
			IpAddr:  ip,
			IpStr:   n.GetAddr(),
			Time:    n.GetTime(),
			Port:    n.GetPort(),
			ID:      n.GetID(),
			NKNaddr: fmt.Sprintf("%x", n.GetChordAddr()),
		})
	}
	return addrs, uint(len(addrs))
}

func (nm *nbrNodes) GetNeighborHeights() ([]uint32, uint) {
	heights := []uint32{}
	for _, n := range nm.GetNeighborNoder() {
		heights = append(heights, n.GetHeight())
	}
	return heights, uint(len(heights))
}

func (nm *nbrNodes) GetNeighborNoder() []protocol.Noder {
	nodes := []protocol.Noder{}
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}

func (nm *nbrNodes) GetSyncFinishedNeighbors() []protocol.Noder {
	var nodes []protocol.Noder
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetSyncState() == protocol.PersistFinished {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}
