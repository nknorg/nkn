package node

import (
	"sync"
	"time"

	cryptoUtil "github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/util"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"github.com/nknorg/nnet/overlay/chord"
)

const (
	maxNumRandomNeighbors         = 8
	randomNeighborConnectInterval = 5 * config.ConsensusTimeout
)

// The neighbor node list
type nbrNodes struct {
	List            sync.Map
	randomNeighbors []string
}

func newNbrNodes() nbrNodes {
	return nbrNodes{
		randomNeighbors: make([]string, 0),
	}
}

func (nm *nbrNodes) GetNbrNode(id string) *RemoteNode {
	v, ok := nm.List.Load(id)
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

func (nm *nbrNodes) DelNbrNode(id string) {
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

	nbr := localNode.GetNbrNode(chordIDToNodeID(nnetRemoteNode.Id))
	return nbr
}

func (localNode *LocalNode) startConnectingToRandomNeighbors() {
	if maxNumRandomNeighbors == 0 {
		return
	}

	c, ok := localNode.nnet.Network.(*chord.Chord)
	if !ok {
		panic("Overlay is not chord")
	}

	for {
		time.Sleep(util.RandDuration(randomNeighborConnectInterval, 1.0/3.0))

		if len(localNode.randomNeighbors) >= maxNumRandomNeighbors {
			nbr := localNode.GetNbrNode(localNode.randomNeighbors[0])
			if nbr != nil && !nbr.nnetNode.IsStopped() {
				c.MaybeStopRemoteNode(nbr.nnetNode)
			}
			localNode.randomNeighbors = localNode.randomNeighbors[1:]
		}

		randID := cryptoUtil.RandomBytes(config.NodeIDBytes)

		succs, err := c.FindSuccessors(randID, 1)
		if err != nil {
			log.Errorf("Find random neighbor at key %x error: %v", randID, err)
			continue
		}

		if len(succs) == 0 {
			log.Errorf("Find no random neighbor at key %x", randID)
			continue
		}

		err = c.Connect(succs[0])
		if err != nil {
			log.Errorf("Connect to random neighbor at key %x error: %v", randID, err)
			continue
		}

		localNode.randomNeighbors = append(localNode.randomNeighbors, chordIDToNodeID(succs[0].Id))

		log.Infof("Connect to random neighbor %x@%s", succs[0].Id, succs[0].Addr)
	}
}
