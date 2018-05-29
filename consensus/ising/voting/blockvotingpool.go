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
	sendPool    map[uint32]map[uint64]Uint256 // record voting history
	receivePool map[uint32]map[uint64]Uint256 // received votes from neighbor
	mind        map[uint32]Uint256            // current idea
	totalWeight int
}

func NewBlockVotingPool(totalWeight int) *BlockVotingPool {
	return &BlockVotingPool{
		sendPool:    make(map[uint32]map[uint64]Uint256),
		receivePool: make(map[uint32]map[uint64]Uint256),
		mind:        make(map[uint32]Uint256),
		totalWeight: totalWeight,
	}
}

func (bvp *BlockVotingPool) GetMind(height uint32) (Uint256, bool) {
	bvp.RLock()
	defer bvp.RUnlock()

	if mind, ok := bvp.mind[height]; ok {
		return mind, true
	} else {
		return Uint256{}, false
	}
}

func (bvp *BlockVotingPool) ChangeMind(height uint32, mind Uint256) map[uint64]Uint256 {
	bvp.Lock()
	bvp.mind[height] = mind
	bvp.Unlock()

	// add self vote to pool
	bvp.AddToReceivePool(height, 0, mind)

	bvp.RLock()
	defer bvp.RUnlock()

	return bvp.sendPool[height]
}

func (bvp *BlockVotingPool) AddToReceivePool(height uint32, nid uint64, hash Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	if _, ok := bvp.receivePool[height]; !ok {
		bvp.receivePool[height] = make(map[uint64]Uint256)
	}

	bvp.receivePool[height][nid] = hash
}

func (bvp *BlockVotingPool) GetReceivePool(height uint32) map[uint64]Uint256 {
	bvp.RLock()
	defer bvp.RUnlock()

	return bvp.receivePool[height]
}

func (bvp *BlockVotingPool) AddToSendPool(height uint32, nid uint64, hash Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	if _, ok := bvp.sendPool[height]; !ok {
		bvp.sendPool[height] = make(map[uint64]Uint256)
	}
	if _, ok := bvp.sendPool[height][nid]; !ok {
		bvp.sendPool[height][nid] = hash
	}
}

func (bvp *BlockVotingPool) VoteCounting(height uint32) (*Uint256, error) {
	bvp.RLock()
	defer bvp.RUnlock()

	m := make(map[Uint256]int)
	// vote counting
	for _, hash := range bvp.receivePool[height] {
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
	// returns hash which got >=50% votes
	for hash, count := range m {
		if 2*count >= bvp.totalWeight {
			return &hash, nil
		}
	}

	return nil, errors.New("invalid votes")
}

func (bvp *BlockVotingPool) Reset(totalWeight int) {
	bvp.totalWeight = totalWeight
}
