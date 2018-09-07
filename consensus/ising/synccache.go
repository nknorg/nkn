package ising

import (
	"fmt"
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

type BlockInfo struct {
	block       *ledger.Block // cached block
	receiveTime int64         // time when receive block
	votes       int           // votes count
}

// BlockWithVotes describes cached block with votes got from neighbors.
type BlockWithVotes struct {
	blockInfo []*BlockInfo // cached block info
	bestBlock *BlockInfo   // the block which got enough votes
}

// SyncCache cached blocks sent by block proposer when wait for block syncing finished.
type SyncCache struct {
	sync.RWMutex
	currHeight      uint32
	startHeight     uint32
	nextHeight      uint32
	consensusHeight uint32
	blockCache      map[uint32]*BlockWithVotes
	voteCache       map[uint32]map[uint64]Uint256
	timeLock        *TimeLock
}

func NewSyncBlockCache() *SyncCache {
	return &SyncCache{
		blockCache: make(map[uint32]*BlockWithVotes),
		voteCache:  make(map[uint32]map[uint64]Uint256),
		timeLock:   NewTimeLock(),
	}
}

// BlockInSyncCache returns block info and true is the block exists in cache.
func (sc *SyncCache) BlockInSyncCache(hash Uint256, height uint32) (*BlockInfo, bool) {
	if blockWithVotes, ok := sc.blockCache[height]; ok {
		for _, b := range blockWithVotes.blockInfo {
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

// GetBlockFromSyncCache returns cached block by height.
func (sc *SyncCache) GetBlockFromSyncCache(height uint32) (*ledger.VBlock, error) {
	err := sc.timeLock.WaitForTimeout(height)
	if err != nil {
		return nil, err
	}

	sc.RLock()
	defer sc.RUnlock()
	if _, ok := sc.blockCache[height]; !ok {
		return nil, fmt.Errorf("no block in cache for height: %d", height)
	}

	// check if there's a block got enough votes
	if sc.blockCache[height].bestBlock == nil {
		return nil, fmt.Errorf("ambiguous block for height: %d", height)
	}

	vBlock := &ledger.VBlock{
		Block:       sc.blockCache[height].bestBlock.block,
		ReceiveTime: sc.blockCache[height].bestBlock.receiveTime,
	}

	return vBlock, nil
}

// AddBlockToSyncCache caches received block and the voter count when receive block.
// Returns nil if block already existed.
func (sc *SyncCache) AddBlockToSyncCache(block *ledger.Block) error {
	sc.Lock()
	defer sc.Unlock()

	hash := block.Hash()
	blockHeight := block.Header.Height
	if _, exist := sc.BlockInSyncCache(hash, blockHeight); exist {
		return nil
	}

	if len(sc.blockCache) == 0 {
		// cached block height [min height, curr height]
		sc.startHeight = blockHeight
		sc.nextHeight = blockHeight + 1
		sc.currHeight = blockHeight
	} else {
		if blockHeight == sc.nextHeight {
			sc.currHeight++
			sc.nextHeight++
		} else if blockHeight != sc.currHeight {
			return fmt.Errorf("adding block which height is invalid, expected: %d or %d, received: %d",
				sc.currHeight, sc.nextHeight, blockHeight)
		}
	}

	blockInfo := &BlockInfo{
		block:       block,
		receiveTime: time.Now().Unix(),
	}
	// analyse cached votes when add new block
	if voteInfo, ok := sc.voteCache[blockHeight]; ok {
		log.Infof("AddBlockToSyncCache: receive block: %s, %d has voted first",
			BytesToHexString(hash.ToArrayReverse()), len(sc.voteCache[blockHeight]))
		for _, h := range voteInfo {
			if hash.CompareTo(h) == 0 {
				blockInfo.votes++
				log.Infof("append vote for block: %s, totally got %d votes: %d",
					BytesToHexString(hash.ToArrayReverse()), blockInfo.votes)
				if 2*blockInfo.votes > len(sc.voteCache[blockHeight]) {
					if _, ok := sc.blockCache[blockHeight]; !ok {
						sc.blockCache[blockHeight] = &BlockWithVotes{}
					}
					sc.blockCache[blockHeight].bestBlock = blockInfo
				}
			}
		}
	} else {
		log.Infof("AddBlockToSyncCache: receive block: %s, have not received vote for it",
			BytesToHexString(hash.ToArrayReverse()))
	}

	// cache new block
	if b, ok := sc.blockCache[blockHeight]; !ok {
		blockWithVotes := &BlockWithVotes{
			blockInfo: []*BlockInfo{blockInfo},
		}
		sc.blockCache[blockHeight] = blockWithVotes
		// set time lock when receive block which has new height
		sc.timeLock.LockForHeight(blockHeight, time.NewTimer(WaitingForVotingFinished))
	} else {
		b.blockInfo = append(b.blockInfo, blockInfo)
	}

	return nil
}

// RemoveBlockFromCache removes cached block which height is minimum.
func (sc *SyncCache) RemoveBlockFromCache(height uint32) error {
	sc.Lock()
	defer sc.Unlock()

	if height != sc.startHeight {
		return errors.New("the height to be removed is not the start height")
	}

	// remove block from block cache
	if _, ok := sc.blockCache[height]; ok {
		delete(sc.blockCache, height)
		if len(sc.blockCache) == 0 {
			sc.startHeight = 0
			sc.currHeight = 0
			sc.nextHeight = 0
		} else {
			sc.startHeight++
		}
	}
	// remove votes from vote cache
	if _, ok := sc.voteCache[height]; ok {
		delete(sc.voteCache, height)
	}

	return nil
}

// AddVoteForBlock adds vote for block when receive valid proposal.
func (sc *SyncCache) AddVoteForBlock(hash Uint256, height uint32, voter uint64) error {
	sc.Lock()
	defer sc.Unlock()

	// cache block votes
	if _, ok := sc.voteCache[height]; !ok {
		sc.voteCache[height] = make(map[uint64]Uint256)
	}
	log.Infof("AddVoteForBlock: receive vote for block: %s, height: %d, voter: %d",
		BytesToHexString(hash.ToArrayReverse()), height, voter)
	sc.voteCache[height][voter] = hash

	if blockInfo, ok := sc.BlockInSyncCache(hash, height); ok {
		// if voted block existed in cache then increase vote
		blockInfo.votes++
		log.Infof("AddVoteForBlock: block: %s already exist, block got %d votes, votes for height %d: %d",
			BytesToHexString(hash.ToArrayReverse()), blockInfo.votes, height, len(sc.voteCache[height]))
		if 2*blockInfo.votes > len(sc.voteCache[height]) {
			sc.blockCache[height].bestBlock = blockInfo
		}
	} else {
		log.Infof("AddVoteForBlock: block: %s doesn't exist, cache vote only",
			BytesToHexString(hash.ToArrayReverse()))
	}

	return nil
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
		if 2*blockInfo.votes <= len(sc.voteCache[height]) {
			sc.blockCache[height].bestBlock = nil
		}
	}
	// add vote for new mind if block exists
	if blockInfo, ok := sc.BlockInSyncCache(hash, height); ok {
		blockInfo.votes++
		if 2*blockInfo.votes > len(sc.voteCache[height]) {
			sc.blockCache[height].bestBlock = blockInfo
		}
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
