package ising

import (
	"fmt"
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

const (
	MinSyncVotesNum = 3
)

type BlockInfo struct {
	block       *ledger.Block // cached block
	receiveTime int64         // time when receive block
	votes       int           // votes count
}

// SyncCache cached blocks sent by block proposer when wait for block syncing finished.
type SyncCache struct {
	sync.RWMutex
	minHeight       uint32
	maxHeight       uint32
	consensusHeight uint32
	blockCache      map[uint32][]*BlockInfo
	voteCache       map[uint32]map[uint64]Uint256
	timeLock        *TimeLock
}

func NewSyncBlockCache() *SyncCache {
	return &SyncCache{
		blockCache: make(map[uint32][]*BlockInfo),
		voteCache:  make(map[uint32]map[uint64]Uint256),
		timeLock:   NewTimeLock(),
	}
}

// BlockInSyncCache returns block info and true is the block exists in cache.
func (sc *SyncCache) BlockInSyncCache(hash Uint256, height uint32) (*BlockInfo, bool) {
	if blocks, ok := sc.blockCache[height]; ok {
		for _, b := range blocks {
			if hash.CompareTo(b.block.Hash()) == 0 {
				return b, true
			}
		}
	}

	return nil, false
}

// CachedBlockHeight returns cached block height.
func (sc *SyncCache) CachedBlockHeight() int {
	sc.RLock()
	defer sc.RUnlock()

	return len(sc.blockCache)
}

func (sc *SyncCache) GetBlock(height uint32, hash *Uint256) (*ledger.Block, error) {
	sc.RLock()
	defer sc.RUnlock()

	if b, ok := sc.blockCache[height]; !ok || b == nil {
		return nil, fmt.Errorf("no block in cache for height: %d when get block", height)
	} else {
		for _, v := range b {
			if hash.CompareTo(v.block.Hash()) == 0 {
				return v.block, nil
			}
		}
		return nil, fmt.Errorf("no block in cache for hash: %s when get block",
			BytesToHexString(hash.ToArrayReverse()))
	}
}

// WaitBlockVotingFinished returns cached block by height.
func (sc *SyncCache) WaitBlockVotingFinished(height uint32) (*ledger.VBlock, error) {
	err := sc.timeLock.WaitForTimeout(height)
	if err != nil {
		return nil, err
	}

	sc.RLock()
	defer sc.RUnlock()
	if b, ok := sc.blockCache[height]; !ok || b == nil {
		return nil, fmt.Errorf("no block in cache for height: %d", height)
	}
	totalVotes := len(sc.voteCache[height])
	if totalVotes < MinSyncVotesNum {
		return nil, fmt.Errorf("total vote is not enough for height: %d, got %d votes", height, totalVotes)
	}
	var bestBlock *BlockInfo
	for _, b := range sc.blockCache[height] {
		if 2*b.votes > totalVotes {
			bestBlock = b
		}
	}
	if bestBlock == nil {
		return nil, fmt.Errorf("no best block in cache for height: %d", height)
	}
	vBlock := &ledger.VBlock{
		Block:       bestBlock.block,
		ReceiveTime: bestBlock.receiveTime,
	}

	return vBlock, nil
}

// AddBlockToSyncCache caches received block and the voter count when receive block.
// Returns nil if block already existed.
func (sc *SyncCache) AddBlockToSyncCache(block *ledger.Block, rtime int64) error {
	sc.Lock()
	defer sc.Unlock()

	hash := block.Hash()
	blockHeight := block.Header.Height
	if _, exist := sc.BlockInSyncCache(hash, blockHeight); exist {
		return nil
	}

	if len(sc.blockCache) == 0 {
		// cached block height [min height, max height]
		sc.minHeight = blockHeight
		sc.maxHeight = blockHeight
	} else {
		if blockHeight > sc.maxHeight {
			sc.maxHeight = blockHeight
		}
		if blockHeight < sc.minHeight {
			sc.minHeight = blockHeight
		}
	}

	blockInfo := &BlockInfo{
		block:       block,
		receiveTime: rtime,
	}
	// analyse cached votes when add new block
	if voteInfo, ok := sc.voteCache[blockHeight]; ok {
		log.Infof("AddBlockToSyncCache: receive block: %s, %d has voted first",
			BytesToHexString(hash.ToArrayReverse()), len(sc.voteCache[blockHeight]))
		for _, h := range voteInfo {
			if hash.CompareTo(h) == 0 {
				blockInfo.votes++
				log.Infof("append vote for block: %s, totally got %d votes",
					BytesToHexString(hash.ToArrayReverse()), blockInfo.votes)
			}
		}
	} else {
		log.Infof("AddBlockToSyncCache: receive block: %s, have not received vote for it",
			BytesToHexString(hash.ToArrayReverse()))
	}

	// cache new block
	if _, ok := sc.blockCache[blockHeight]; !ok {
		sc.blockCache[blockHeight] = []*BlockInfo{blockInfo}
		// set time lock when receive block which has new height
		sc.timeLock.LockForHeight(blockHeight, time.NewTimer(WaitingForVotingFinished))
	} else {
		sc.blockCache[blockHeight] = append(sc.blockCache[blockHeight], blockInfo)
	}

	return nil
}

// RemoveBlockFromCache removes cached block which height is minimum.
func (sc *SyncCache) RemoveBlockFromCache(height uint32) error {
	sc.Lock()
	defer sc.Unlock()

	// remove block from block cache
	if _, ok := sc.blockCache[height]; ok {
		delete(sc.blockCache, height)
	}
	// remove votes from vote cache
	if _, ok := sc.voteCache[height]; ok {
		delete(sc.voteCache, height)
	}

	return nil
}

// AddVoteForBlock adds vote for block when receive valid proposal.
func (sc *SyncCache) AddVoteForBlock(hash Uint256, height uint32, voter uint64) bool {
	sc.Lock()
	defer sc.Unlock()

	// cache block votes
	if _, ok := sc.voteCache[height]; !ok {
		sc.voteCache[height] = make(map[uint64]Uint256)
	}
	log.Infof("AddVoteForBlock: receive vote for block: %s, height: %d, voter: %d",
		BytesToHexString(hash.ToArrayReverse()), height, voter)
	sc.voteCache[height][voter] = hash

	exist := false
	if blockInfo, ok := sc.BlockInSyncCache(hash, height); ok {
		// if voted block existed in cache then increase vote
		blockInfo.votes++
		log.Infof("AddVoteForBlock: block: %s already exist, block got %d votes, votes for height %d: %d",
			BytesToHexString(hash.ToArrayReverse()), blockInfo.votes, height, len(sc.voteCache[height]))
		exist = true
	} else {
		log.Infof("AddVoteForBlock: block: %s doesn't exist, cache vote only",
			BytesToHexString(hash.ToArrayReverse()))
	}

	return exist
}

// ChangeVoteForBlock change vote for block when receive valid mind changing.
func (sc *SyncCache) ChangeVoteForBlock(hash Uint256, height uint32, voter uint64) error {
	sc.Lock()
	defer sc.Unlock()

	voteInfo, ok := sc.voteCache[height]
	if !ok {
		return fmt.Errorf("no previous vote for height: %d", height)
	}
	previous, ok := voteInfo[voter]
	if !ok {
		return fmt.Errorf("no previous vote for block: %s", BytesToHexString(hash.ToArrayReverse()))
	}
	// change vote
	voteInfo[voter] = hash
	// remove previous vote if previous exists
	if blockInfo, ok := sc.BlockInSyncCache(previous, height); ok {
		blockInfo.votes--
	}
	// add vote for new mind if block exists
	if blockInfo, ok := sc.BlockInSyncCache(hash, height); ok {
		blockInfo.votes++
	}

	return nil
}

func (sc *SyncCache) SetConsensusHeight(height uint32) {
	sc.Lock()
	defer sc.Unlock()

	// set consensus height if it has not been set
	if sc.consensusHeight == 0 {
		sc.consensusHeight = height
	}
}
