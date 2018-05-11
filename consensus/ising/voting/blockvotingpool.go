package voting

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

type BlockVotingPool struct {
	sync.RWMutex
	sendPool    map[uint64]map[uint32]Uint256 // record voting history
	receivePool map[uint64]map[uint32]Uint256 // received votes from neighbor
	mind        map[uint32]Uint256            // current idea
	totalWeight int
}

func NewBlockVotingPool(totalWeight int) *BlockVotingPool {
	return &BlockVotingPool{
		sendPool:    make(map[uint64]map[uint32]Uint256),
		receivePool: make(map[uint64]map[uint32]Uint256),
		mind:        make(map[uint32]Uint256),
		totalWeight: totalWeight,
	}
}

func (bvp *BlockVotingPool) SetMind(height uint32, mind Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	if hash, ok := bvp.mind[height]; !ok {
		bvp.mind[height] = mind
	} else {
		if mind.CompareTo(hash) == -1 {
			bvp.mind[height] = mind
		}
	}
}

func (bvp *BlockVotingPool) GetMind(height uint32) Uint256 {
	bvp.RLock()
	defer bvp.RUnlock()

	return bvp.mind[height]
}

func (bvp *BlockVotingPool) ChangeMind(height uint32, mind Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	bvp.mind[height] = mind
	// TODO: broadcast
}

func (bvp *BlockVotingPool) AddToReceivePool(nid uint64, height uint32, hash Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	if _, ok := bvp.receivePool[nid]; !ok {
		bvp.receivePool[nid] = make(map[uint32]Uint256)
	}
	if _, ok := bvp.receivePool[nid][height]; !ok {
		bvp.receivePool[nid][height] = hash
	}
}

func (bvp *BlockVotingPool) AddToSendPool(nid uint64, height uint32, hash Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	if _, ok := bvp.sendPool[nid]; !ok {
		bvp.sendPool[nid] = make(map[uint32]Uint256)
	}
	if _, ok := bvp.sendPool[nid][height]; !ok {
		bvp.sendPool[nid][height] = hash
	}
}

func (bvp *BlockVotingPool) VoteCounting(height uint32) (*Uint256, error) {
	bvp.RLock()
	defer bvp.RUnlock()

	m := make(map[Uint256]int)
	// counting votes from neighbors
	for _, item := range bvp.receivePool {
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
	if selfMind, ok := bvp.mind[height]; !ok {
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
		if 2*count >= bvp.totalWeight {
			return &hash, nil
		}
	}

	return nil, errors.New("invalid votes")
}

func (bvp *BlockVotingPool) NeedChangeMind(height uint32, hash Uint256) bool {
	if hash.CompareTo(bvp.GetMind(height)) == 0 {
		return false
	}

	return true
}

func (bvp *BlockVotingPool) SpreadNewMind() {
	//TODO: notify neighbors in sendpool
}

func (bvp *BlockVotingPool) Reset(totalWeight int) {
	bvp.totalWeight = totalWeight
}
