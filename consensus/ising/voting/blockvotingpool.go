package voting

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
)

type BlockVotingPool struct {
	sync.RWMutex
	sendPool    map[uint32]map[uint64]Uint256   // record voting history
	receivePool map[uint32]map[uint64]*VoteInfo // received votes from neighbor
	mind        map[uint32]Uint256              // current idea
	totalWeight int
}

func NewBlockVotingPool() *BlockVotingPool {
	return &BlockVotingPool{
		sendPool:    make(map[uint32]map[uint64]Uint256),
		receivePool: make(map[uint32]map[uint64]*VoteInfo),
		mind:        make(map[uint32]Uint256),
		totalWeight: 0,
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

func (bvp *BlockVotingPool) SetMind(height uint32, mind Uint256) map[uint64]Uint256 {
	bvp.Lock()
	defer bvp.Unlock()

	bvp.mind[height] = mind

	return bvp.sendPool[height]
}

func (bvp *BlockVotingPool) HasReceivedVoteFrom(height uint32, nodeID uint64) bool {
	bvp.RLock()
	defer bvp.RUnlock()

	if _, ok := bvp.receivePool[height]; !ok {
		return false
	}
	if _, ok := bvp.receivePool[height][nodeID]; !ok {
		return false
	}

	return true
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

func (bvp *BlockVotingPool) AddToReceivePool(height uint32, nodeID uint64, weight int, hash Uint256) {
	bvp.Lock()
	defer bvp.Unlock()

	bvp.addToReceivePool(height, nodeID, weight, hash)
}

func (bvp *BlockVotingPool) AddVoteThenCounting(height uint32, nodeID uint64, weight int, hash Uint256) (*Uint256, error) {
	bvp.Lock()
	defer bvp.Unlock()

	// add the vote to pool
	bvp.addToReceivePool(height, nodeID, weight, hash)

	// return when voter is not enough
	if len(bvp.receivePool[height]) < MinConsensusVotesNum {
		return nil, errors.New("voter is not enough")
	}

	// vote counting
	m := make(map[Uint256]int)
	for _, voteInfo := range bvp.receivePool[height] {
		vHash := voteInfo.votedHash
		vWeight := voteInfo.voterWeight
		if _, ok := m[vHash]; ok {
			m[vHash] += vWeight
		} else {
			m[vHash] = vWeight
		}
	}

	// returns hash which got >50% votes
	for hash, count := range m {
		if 2*count > bvp.totalWeight {
			log.Debugf("block voting result: %s, (%d votes / %d current total votes)",
				BytesToHexString(hash.ToArrayReverse()), count, bvp.totalWeight)
			return &hash, nil
		}
	}

	return nil, errors.New("invalid votes")
}

func (bvp *BlockVotingPool) Reset() {
	bvp.totalWeight = 0
}

func (bvp *BlockVotingPool) addToReceivePool(height uint32, nodeID uint64, weight int, hash Uint256) {
	if _, ok := bvp.receivePool[height]; !ok {
		bvp.receivePool[height] = make(map[uint64]*VoteInfo)
	}
	// if it's first time to receive vote from neighbor then increase the total voting weight
	if _, ok := bvp.receivePool[height][nodeID]; !ok {
		bvp.totalWeight += weight
	}
	voteInfo := &VoteInfo{
		votedHash:   hash,
		voterWeight: weight,
	}
	// update receive pool
	bvp.receivePool[height][nodeID] = voteInfo
}
