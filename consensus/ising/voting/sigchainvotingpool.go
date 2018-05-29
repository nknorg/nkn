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
	sendPool    map[uint32]map[uint64]Uint256 // record voting history
	receivePool map[uint32]map[uint64]Uint256 // received votes from neighbor
	mind        map[uint32]Uint256            // current idea
	totalWeight int                           // total voting weight
}

func NewSigChainVotingPool(totalWeight int) *SigChainVotingPool {
	return &SigChainVotingPool{
		sendPool:    make(map[uint32]map[uint64]Uint256),
		receivePool: make(map[uint32]map[uint64]Uint256),
		mind:        make(map[uint32]Uint256),
		totalWeight: totalWeight,
	}
}

func (scvp *SigChainVotingPool) GetMind(height uint32) (Uint256, bool) {
	scvp.RLock()
	defer scvp.RUnlock()

	if mind, ok := scvp.mind[height]; ok {
		return mind, true
	} else {
		return Uint256{}, false
	}
}

func (scvp *SigChainVotingPool) ChangeMind(height uint32, mind Uint256) map[uint64]Uint256{
	scvp.Lock()
	scvp.mind[height] = mind
	scvp.Unlock()

	// add self vote to pool
	scvp.AddToReceivePool(height, 0, mind)

	scvp.RLock()
	defer scvp.RUnlock()

	return scvp.sendPool[height]
}

func (scvp *SigChainVotingPool) AddToReceivePool(height uint32, nid uint64, hash Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	if _, ok := scvp.receivePool[height]; !ok {
		scvp.receivePool[height] = make(map[uint64]Uint256)
	}
	scvp.receivePool[height][nid] = hash
}

func (bvp *SigChainVotingPool) GetReceivePool(height uint32) map[uint64]Uint256 {
	bvp.RLock()
	defer bvp.RUnlock()

	return bvp.receivePool[height]
}

func (scvp *SigChainVotingPool) AddToSendPool(height uint32, nid uint64, hash Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	if _, ok := scvp.sendPool[height]; !ok {
		scvp.sendPool[height] = make(map[uint64]Uint256)
	}
	if _, ok := scvp.sendPool[height][nid]; !ok {
		scvp.sendPool[height][nid] = hash
	}
}

func (scvp *SigChainVotingPool) VoteCounting(height uint32) (*Uint256, error) {
	scvp.RLock()
	defer scvp.RUnlock()

	m := make(map[Uint256]int)
	// vote counting
	for _, hash := range scvp.receivePool[height] {
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
		if 2*count >= scvp.totalWeight {
			return &hash, nil
		}
	}

	return nil, errors.New("invalid votes")
}

func (scvp *SigChainVotingPool) Reset(totalWeight int) {
	scvp.totalWeight = totalWeight
}
