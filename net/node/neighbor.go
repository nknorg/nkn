package node

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	. "github.com/nknorg/nkn/net/protocol"
)

// The neighbor node list
type nbrNodes struct {
	List sync.Map
}

func (nm *nbrNodes) Broadcast(buf []byte) {
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetState() == ESTABLISH && n.relay == true {
			n.Tx(buf)
		}
		return true
	})
}

func (nm *nbrNodes) NodeExisted(uid uint64) bool {
	_, ok := nm.List.Load(uid)
	return ok
}

func (nm *nbrNodes) AddNbrNode(n Noder) {
	node, ok := n.(*node)
	if !ok {
		fmt.Println("Convert the noder error when add node")
		return
	}
	nm.List.LoadOrStore(n.GetID(), node)
}

func (nm *nbrNodes) DelNbrNode(id uint64) (Noder, bool) {
	v, ok := nm.List.Load(id)
	if !ok {
		return nil, false
	}
	n, ok := v.(*node)
	nm.List.Delete(id)
	return n, true
}

func (nm *nbrNodes) GetConnectionCnt() uint {
	return uint(len(nm.GetNeighborNoder()))
}

func (nm *nbrNodes) init() {
}

func (nm *nbrNodes) NodeEstablished(id uint64) bool {
	v, ok := nm.List.Load(id)
	if !ok {
		return false
	}

	n, ok := v.(*node)
	if !ok {
		return false
	}

	if n.GetState() != ESTABLISH {
		return false
	}

	return true
}

func (nm *nbrNodes) GetNeighborAddrs() ([]NodeAddr, uint) {
	var addrs []NodeAddr
	for _, n := range nm.GetNeighborNoder() {
		ip, _ := n.GetAddr16()
		addrs = append(addrs, NodeAddr{
			IpAddr:   ip,
			Time:     n.GetTime(),
			Services: n.Services(),
			Port:     n.GetPort(),
			ID:       n.GetID(),
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

func (nm *nbrNodes) GetActiveNeighbors() []Noder {
	nodes := []Noder{}
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetState() != INACTIVITY {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}

func (nm *nbrNodes) GetNeighborNoder() []Noder {
	nodes := []Noder{}
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetState() == ESTABLISH {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}

func (nm *nbrNodes) GetSyncFinishedNeighbors() []Noder {
	var nodes []Noder
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetState() == ESTABLISH && n.GetSyncState() == PersistFinished {
			nodes = append(nodes, n)
		}
		return true
	})
	return nodes
}

func (nm *nbrNodes) GetNbrNodeCnt() uint32 {
	return uint32(len(nm.GetNeighborNoder()))
}

func (nm *nbrNodes) GetNeighborByAddr(addr string) Noder {
	var nbr Noder
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetState() == ESTABLISH && n.GetAddrStr() == addr {
			nbr = n
			return false
		}
		return true
	})
	return nbr
}

func (nm *nbrNodes) GetNeighborByChordAddr(chordAddr []byte) Noder {
	var nbr Noder
	nm.List.Range(func(key, value interface{}) bool {
		n, ok := value.(*node)
		if ok && n.GetState() == ESTABLISH && bytes.Compare(n.GetChordAddr(), chordAddr) == 0 {
			nbr = n
			return false
		}
		return true
	})
	return nbr
}

func (node *node) IsAddrInNeighbors(addr string) bool {
	neighbor := node.GetNeighborByAddr(addr)
	if neighbor != nil {
		return true
	}
	return false
}

func (node *node) IsChordAddrInNeighbors(chordAddr []byte) bool {
	neighbor := node.GetNeighborByChordAddr(chordAddr)
	if neighbor != nil {
		return true
	}
	return false
}

func (node *node) ShouldChordAddrInNeighbors(addr []byte) (bool, error) {
	chordNode, err := node.ring.GetFirstVnode()
	if err != nil {
		return false, err
	}
	if chordNode == nil {
		return false, errors.New("No chord node binded")
	}
	return chordNode.ShouldAddrInNeighbors(addr), nil
}
