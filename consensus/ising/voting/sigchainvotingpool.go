package voting

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

type SigChainVotingPool struct {
	sync.RWMutex
	sendPool    map[uint64]map[uint32]Uint256 // record voting history
	receivePool map[uint64]map[uint32]Uint256 // received votes from neighbor
	mind        map[uint32]Uint256            // current idea
	totalWeight int                           // total voting weight
}

func NewSigChainVotingPool(totalWeight int) *SigChainVotingPool {
	return &SigChainVotingPool{
		sendPool:    make(map[uint64]map[uint32]Uint256),
		receivePool: make(map[uint64]map[uint32]Uint256),
		mind:        make(map[uint32]Uint256),
		totalWeight: totalWeight,
	}
}

func (scvp *SigChainVotingPool) SetMind(height uint32, mind Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	if hash, ok := scvp.mind[height]; !ok {
		scvp.mind[height] = mind
	} else {
		if mind.CompareTo(hash) == -1 {
			scvp.mind[height] = mind
		}
	}
}

func (scvp *SigChainVotingPool) GetMind(height uint32) Uint256 {
	scvp.RLock()
	defer scvp.RUnlock()

	return scvp.mind[height]
}

func (scvp *SigChainVotingPool) ChangeMind(height uint32, mind Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	scvp.mind[height] = mind
	// TODO: broadcast
}

func (scvp *SigChainVotingPool) AddToReceivePool(nid uint64, height uint32, hash Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	if _, ok := scvp.receivePool[nid]; !ok {
		scvp.receivePool[nid] = make(map[uint32]Uint256)
	}
	if _, ok := scvp.receivePool[nid][height]; !ok {
		scvp.receivePool[nid][height] = hash
	}
}

func (scvp *SigChainVotingPool) AddToSendPool(nid uint64, height uint32, hash Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	if _, ok := scvp.sendPool[nid]; !ok {
		scvp.sendPool[nid] = make(map[uint32]Uint256)
	}
	if _, ok := scvp.sendPool[nid][height]; !ok {
		scvp.sendPool[nid][height] = hash
	}
}

func (scvp *SigChainVotingPool) VoteCounting(height uint32) (*Uint256, error) {
	scvp.RLock()
	defer scvp.RUnlock()

	m := make(map[Uint256]int)
	// counting votes from neighbors
	for _, item := range scvp.receivePool {
		for h, hash := range item {
			if h == height {
				weight, err := ledger.DefaultLedger.Store.GetVotingWeight(Uint160{})
				if err != nil {
					log.Warn("get voter weight error")
					return nil, errors.New("get voter weight error")
				}
				if _, ok := m[hash]; ok {
					m[hash] += weight
				} else {
					m[hash] = weight
				}
			}
		}
	}
	// self vote
	if selfMind, ok := scvp.mind[height]; !ok {
		return nil, errors.New("self mind missing")
	} else {
		selfWeight, err := ledger.DefaultLedger.Store.GetVotingWeight(Uint160{})
		if err != nil {
			log.Warn("get self weight error")
			return nil, errors.New("get self weight error")
		}
		if _, ok := m[selfMind]; ok {
			m[selfMind] += selfWeight
		} else {
			m[selfMind] = selfWeight
		}
	}
	// returns hash which got >=50% votes
	for hash, count := range m {
		if 2*count >= scvp.totalWeight {
			return &hash, nil
		}
	}

	return nil, errors.New("invalid votes")
}

func (scvp *SigChainVotingPool) NeedChangeMind(height uint32, hash Uint256) bool {
	if hash.CompareTo(scvp.GetMind(height)) == 0 {
		return false
	}

	return true
}

func (scvp *SigChainVotingPool) SpreadNewMind() {
	//TODO: notify neighbors in sendpool
}

func (scvp *SigChainVotingPool) Reset(totalWeight int) {
	scvp.totalWeight = totalWeight
}
