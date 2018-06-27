package ising

import (
	"sync"

	"errors"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
)

// SyncCache cached blocks sent by block proposer when wait for block syncing finished.
type SyncCache struct {
	sync.RWMutex
	minHeight uint32
	maxHeight uint32
	cache     map[uint32][]*ledger.Block
}

func NewSyncBlockCache() *SyncCache {
	return &SyncCache{
		minHeight: 0,
		maxHeight: 0,
		cache:     make(map[uint32][]*ledger.Block),
	}
}

// BlockInSyncCache returns whether the block exists in cache pool.
func (sc *SyncCache) BlockInSyncCache(hash Uint256, height uint32) bool {
	sc.RLock()
	defer sc.RUnlock()

	if blocks, ok := sc.cache[height]; ok {
		for _, b := range blocks {
			if hash.CompareTo(b.Hash()) == 0 {
				return true
			}
		}
	}

	return false
}

// CachedBlockHeight returns cached block height.
func (sc *SyncCache) CachedBlockHeight() int {
	sc.RLock()
	defer sc.RUnlock()

	return len(sc.cache)
}

// GetBlockFromSyncCache returns cached block which height is minimum.
func (sc *SyncCache) GetBlockFromSyncCache() *ledger.Block {
	sc.RLock()
	defer sc.RUnlock()

	if blocks, ok := sc.cache[sc.minHeight]; !ok {
		return nil
	} else {
		if len(blocks) == 0 {
			return nil
		}
		// TODO: return block which got enough votes
		return blocks[0]
	}
}

// AddBlockToSyncCache add received block to cache. Returns nil if block already existed.
func (sc *SyncCache) AddBlockToSyncCache(block *ledger.Block) error {
	hash := block.Hash()
	blockHeight := block.Header.Height
	if sc.BlockInSyncCache(hash, blockHeight) {
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
	sc.cache[blockHeight] = append(sc.cache[blockHeight], block)

	return nil
}

// RemoveBlockFromCache removes cached block which height is minimum.
func (sc *SyncCache) RemoveBlockFromCache() error {
	sc.Lock()
	defer sc.Unlock()

	if _, ok := sc.cache[sc.minHeight]; !ok {
		return nil
	}
	delete(sc.cache, sc.minHeight)
	sc.minHeight++

	return nil
}
