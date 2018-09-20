package voting

import (
	"errors"
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/log"
)

type SigChainVotingPool struct {
	sync.RWMutex
	sendPool    map[uint32]map[uint64]Uint256   // record voting history
	receivePool map[uint32]map[uint64]*VoteInfo // received votes from neighbor
	mind        map[uint32]Uint256              // current idea
	totalWeight int                             // total voting weight
}

func NewSigChainVotingPool() *SigChainVotingPool {
	return &SigChainVotingPool{
		sendPool:    make(map[uint32]map[uint64]Uint256),
		receivePool: make(map[uint32]map[uint64]*VoteInfo),
		mind:        make(map[uint32]Uint256),
		totalWeight: 0,
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

func (scvp *SigChainVotingPool) SetMind(height uint32, mind Uint256) map[uint64]Uint256 {
	scvp.Lock()
	defer scvp.Unlock()

	scvp.mind[height] = mind

	return scvp.sendPool[height]
}

func (scvp *SigChainVotingPool) HasReceivedVoteFrom(height uint32, nodeID uint64) bool {
	scvp.RLock()
	defer scvp.RUnlock()

	if _, ok := scvp.receivePool[height]; !ok {
		return false
	}
	if _, ok := scvp.receivePool[height][nodeID]; !ok {
		return false
	}

	return true
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

func (scvp *SigChainVotingPool) AddToReceivePool(height uint32, nodeID uint64, weight int, hash Uint256) {
	scvp.Lock()
	defer scvp.Unlock()

	scvp.addToReceivePool(height, nodeID, weight, hash)
}

func (scvp *SigChainVotingPool) AddVoteThenCounting(height uint32, nodeID uint64, weight int, hash Uint256) (*Uint256, error) {
	scvp.Lock()
	defer scvp.Unlock()

	// add the vote to pool
	scvp.addToReceivePool(height, nodeID, weight, hash)

	// return when voter is not enough
	if len(scvp.receivePool[height]) < MinConsensusVotesNum {
		return nil, errors.New("voter is not enough")
	}

	// vote counting
	m := make(map[Uint256]int)
	for _, voteInfo := range scvp.receivePool[height] {
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
		if 2*count > scvp.totalWeight {
			log.Debugf("transaction voting result: %s, (%d votes / %d current total votes)",
				BytesToHexString(hash.ToArrayReverse()), count, scvp.totalWeight)
			return &hash, nil
		}
	}

	return nil, errors.New("invalid votes")
}

func (scvp *SigChainVotingPool) Reset() {
	scvp.totalWeight = 0
}

func (scvp *SigChainVotingPool) addToReceivePool(height uint32, nodeID uint64, weight int, hash Uint256) {
	if _, ok := scvp.receivePool[height]; !ok {
		scvp.receivePool[height] = make(map[uint64]*VoteInfo)
	}
	// if it's first time to receive vote from neighbor then increase the total voting weight
	if _, ok := scvp.receivePool[height][nodeID]; !ok {
		scvp.totalWeight += weight
	}
	voteInfo := &VoteInfo{
		votedHash:   hash,
		voterWeight: weight,
	}
	// update receive pool
	scvp.receivePool[height][nodeID] = voteInfo
}
