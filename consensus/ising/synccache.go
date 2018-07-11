package ising

import (
	"errors"
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

// BlockWithVotes describes cached block with vote count got from neighbors.
type BlockWithVotes struct {
	block           *ledger.Block // cached block
	voters          []uint64      // block voter ID
	totalVoterCount int           // total voter count when receive block
	isMostVoted     bool          // whether this block get most votes
}

// CacheItem cached blocks for same height
type CacheItem struct {
	blocksWithVote []*BlockWithVotes // cached block with votes
	timeLocker     *time.Timer       // time locker for block, block could be used after it expired
}

// SyncCache cached blocks sent by block proposer when wait for block syncing finished.
type SyncCache struct {
	sync.RWMutex
	minHeight uint32
	maxHeight uint32
	cache     map[uint32]*CacheItem
}

func NewSyncBlockCache() *SyncCache {
	return &SyncCache{
		minHeight: 0,
		maxHeight: 0,
		cache:     make(map[uint32]*CacheItem),
	}
}

// BlockInSyncCache returns the block with voters and true if it exists in cache.
func (sc *SyncCache) BlockInSyncCache(hash Uint256, height uint32) (*BlockWithVotes, bool) {
	sc.RLock()
	defer sc.RUnlock()

	if item, ok := sc.cache[height]; ok {
		for _, b := range item.blocksWithVote {
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

	return len(sc.cache)
}

// GetBlockFromSyncCache returns cached block which height is minimum and its vote count.
func (sc *SyncCache) GetBlockFromSyncCache() *ledger.Block {
	sc.RLock()
	defer sc.RUnlock()

	item, ok := sc.cache[sc.minHeight]
	if !ok {
		return nil
	}
	if len(item.blocksWithVote) == 0 {
		return nil
	}
	select {
	case <-item.timeLocker.C:
		item.timeLocker.Reset(0)
		var mostVotesBlock *BlockWithVotes
		for _, v := range item.blocksWithVote {
			if v.isMostVoted {
				mostVotesBlock = v
				break
			}
		}
		if mostVotesBlock == nil {
			log.Error("inconsistent block voting for height: ", sc.minHeight)
			return nil
		}

		return mostVotesBlock.block
	}
}

// AddBlockToSyncCache caches received block and the voter count when receive block.
// Returns nil if block already existed.
func (sc *SyncCache) AddBlockToSyncCache(block *ledger.Block, totalVoterCount int) error {
	hash := block.Hash()
	blockHeight := block.Header.Height
	if _, exist := sc.BlockInSyncCache(hash, blockHeight); exist {
		return nil
	}

	sc.Lock()
	defer sc.Unlock()
	if len(sc.cache) == 0 {
		sc.minHeight = blockHeight
		sc.maxHeight = blockHeight
	} else {
		if blockHeight != sc.maxHeight+1 {
			return errors.New("adding block which height is invalid")
		}
		sc.maxHeight++
	}
	zeroVoteBlock := &BlockWithVotes{
		block:           block,
		voters:          nil,
		totalVoterCount: totalVoterCount,
		isMostVoted:     false,
	}
	sc.cache[blockHeight] = new(CacheItem)
	sc.cache[blockHeight].timeLocker = time.NewTimer(WaitingForVotingFinished)
	sc.cache[blockHeight].blocksWithVote = append(sc.cache[blockHeight].blocksWithVote, zeroVoteBlock)

	return nil
}

// RemoveBlockFromCache removes cached block which height is minimum.
func (sc *SyncCache) RemoveBlockFromCache() error {
	sc.Lock()
	defer sc.Unlock()

	if item, ok := sc.cache[sc.minHeight]; !ok {
		return nil
	} else {
		item.timeLocker.Stop()
	}
	delete(sc.cache, sc.minHeight)
	sc.minHeight++

	return nil
}

// AddVoteForBlock adds vote for a block when receive valid proposal.
func (sc *SyncCache) AddVoteForBlock(hash Uint256, height uint32, voter uint64) error {
	if b, exist := sc.BlockInSyncCache(hash, height); !exist {
		return errors.New("voted block doesn't exist")
	} else {
		b.voters = append(b.voters, voter)
		if 2*len(b.voters) > b.totalVoterCount {
			log.Infof("got enough votes for block %s in syncing (%d votes / %d neighbors)",
				BytesToHexString(hash.ToArrayReverse()), len(b.voters), b.totalVoterCount)
			b.isMostVoted = true
		}
	}

	return nil
}

// ChangeVoteForBlock change vote for a block when mind changed.
func (sc *SyncCache) ChangeVoteForBlock(hash Uint256, height uint32, voter uint64) error {
	b, exist := sc.BlockInSyncCache(hash, height)
	if !exist {
		return errors.New("voted block doesn't exist when change vote")
	}
	item, ok := sc.cache[height]
	if !ok {
		return errors.New("voted block height doesn't exist when change vote")
	}
	// remove previous vote
	found := false
	for _, b := range item.blocksWithVote {
		for i, v := range b.voters {
			if v == voter {
				found = true
				b.voters = append(b.voters[0:i], b.voters[i+1:]...)
				if 2*len(b.voters) <= b.totalVoterCount && b.isMostVoted {
					log.Infof("got insufficient votes for block %s after changing mind"+
						" (%d votes / %d neighbors)", BytesToHexString(hash.ToArrayReverse()),
						len(b.voters), b.totalVoterCount)
					b.isMostVoted = false
				}
			}
		}
	}
	if !found {
		return errors.New("no previous voted block")
	}
	// add new vote
	b.voters = append(b.voters, voter)
	if 2*len(b.voters) > b.totalVoterCount {
		log.Infof("got enough votes for block %s after changing mind (%d votes / %d neighbors)",
			BytesToHexString(hash.ToArrayReverse()), len(b.voters), b.totalVoterCount)
		b.isMostVoted = true
	}

	return nil
}
