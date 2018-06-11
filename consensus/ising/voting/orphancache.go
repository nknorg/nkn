package voting

import (
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
)

const (
	MaxCachedOrphanBlockHeight = 5
)

type OrphanBlockCache struct {
	sync.RWMutex
	cap    int
	orphan map[uint32][]*ledger.Block
}

func NewOrphanBlockCache() *OrphanBlockCache {
	return &OrphanBlockCache{
		cap:    MaxCachedOrphanBlockHeight,
		orphan: make(map[uint32][]*ledger.Block),
	}
}

// BlockInOrphanPool returns whether the block exists in orphan pool.
func (op *OrphanBlockCache) BlockInOrphanPool(hash Uint256, height uint32) bool {
	op.RLock()
	defer op.RUnlock()

	if blocks, ok := op.orphan[height]; ok {
		for _, b := range blocks {
			if hash.CompareTo(b.Hash()) == 0 {
				return true
			}
		}
	}

	return false
}

// GetMinOrphanHeight returns min orphan block height in orphan pool
func (op *OrphanBlockCache) GetOrphanBlocks(height uint32) []*ledger.Block {
	op.RLock()
	defer op.RUnlock()

	if _, ok := op.orphan[height]; ok {
		return op.orphan[height]
	}

	return nil
}

// RemoveOrphanBlockByHeight removes orphan blocks according to the height.
func (op *OrphanBlockCache) RemoveOrphanBlockByHeight(height uint32) error {
	op.Lock()
	defer op.Unlock()

	if _, ok := op.orphan[height]; !ok {
		return nil
	}
	delete(op.orphan, height)

	return nil
}

// CachedOrphanBlockHeight return cached block height
func (op *OrphanBlockCache) CachedOrphanBlockHeight() int {
	op.RLock()
	defer op.RUnlock()

	return len(op.orphan)
}

// GetMinOrphanHeight returns min orphan block height in orphan pool
func (op *OrphanBlockCache) GetMinOrphanHeight() uint32 {
	op.RLock()
	defer op.RUnlock()

	var minHeight uint32
	for height := range op.orphan {
		minHeight = height
	}
	for height := range op.orphan {
		if height < minHeight {
			minHeight = height
		}
	}

	return minHeight
}

// AddBlockToCache returns nil if block already existed in cache
func (op *OrphanBlockCache) AddBlockToOrphanPool(block *ledger.Block) error {
	hash := block.Hash()
	blockHeight := block.Header.Height
	if op.BlockInOrphanPool(hash, blockHeight) {
		return nil
	}
	if op.CachedOrphanBlockHeight() >= op.cap {
		minHeight := op.GetMinOrphanHeight()
		op.RemoveOrphanBlockByHeight(minHeight)
	}
	op.Lock()
	defer op.Unlock()
	op.orphan[blockHeight] = append(op.orphan[blockHeight], block)

	return nil
}
