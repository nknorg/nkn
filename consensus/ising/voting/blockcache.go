package voting

import (
	"sync"
	"time"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
)

const (
	MaxCachedBlocks = 1000
)

// BlockInfo hold the block which waiting for voting
type BlockInfo struct {
	block    *ledger.Block
	lifetime *time.Timer // expired block will be auto removed
}

type BlockCache struct {
	sync.RWMutex
	cap           int
	currentHeight uint32                //current block height
	cache         map[uint32]*BlockInfo // height and block mapping
	hashes        map[Uint256]uint32    // for block fast searching
}

// When receive new block message from consensus layer, cache it.
func NewCache() *BlockCache {
	blockCache := &BlockCache{
		cap:           MaxCachedBlocks,
		currentHeight: 0,
		cache:         make(map[uint32]*BlockInfo),
		hashes:        make(map[Uint256]uint32),
	}
	go blockCache.Cleanup()

	return blockCache
}

// BlockInCache returns whether the block has been cached.
func (bc *BlockCache) BlockInCache(hash Uint256) bool {
	bc.RLock()
	defer bc.RUnlock()

	if _, ok := bc.hashes[hash]; ok {
		return true
	}

	return false
}

// GetBlockFromCache returns block according to bloch hash passed in.
func (bc *BlockCache) GetBlockFromCache(hash Uint256) *ledger.Block {
	bc.RLock()
	defer bc.RUnlock()

	if i, ok := bc.hashes[hash]; !ok {
		return nil
	} else {
		if j, ok := bc.cache[i]; !ok {
			return nil
		} else {
			return j.block
		}
	}
}

// GetCurrentBlockFromCache returns latest block in cache
func (bc *BlockCache) GetCurrentBlockFromCache() *ledger.Block {
	bc.RLock()
	defer bc.RUnlock()

	if v, ok := bc.cache[bc.currentHeight]; ok {
		return v.block
	}

	return nil
}

// RemoveBlockFromCache return true if the block doesn't exist in cache.
func (bc *BlockCache) RemoveBlockFromCache(hash Uint256) error {
	bc.Lock()
	defer bc.Unlock()

	if bc.BlockInCache(hash) {
		h := bc.hashes[hash]
		delete(bc.cache, h)
		delete(bc.hashes, hash)
		if bc.currentHeight > 0 {
			bc.currentHeight--
		}
	}

	return nil
}

// CachedBlockNum return the block number in cache
func (bc *BlockCache) CachedBlockNum() int {
	bc.RLock()
	defer bc.RUnlock()

	return len(bc.cache)
}

// AddBlockToCache returns nil if block already existed in cache
func (bc *BlockCache) AddBlockToCache(block *ledger.Block) error {
	hash := block.Hash()
	if bc.BlockInCache(hash) {
		return nil
	}
	// TODO FIFO cleanup, if cap space is not enough then
	// remove block from cache according to FIFO
	blockInfo := &BlockInfo{
		block:    block,
		lifetime: time.NewTimer(time.Hour),
	}
	bc.Lock()
	defer bc.Unlock()
	blockHeight := block.Header.Height
	bc.cache[blockHeight] = blockInfo
	bc.hashes[hash] = blockHeight
	bc.currentHeight = blockHeight

	return nil
}

// Cleanup is a background routine used for cleaning up expired block in cache
func (bc *BlockCache) Cleanup() {
	ticket := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticket.C:
			for _, blockInfo := range bc.cache {
				select {
				case <-blockInfo.lifetime.C:
					bc.RemoveBlockFromCache(blockInfo.block.Hash())
				}
			}
		}
	}
}
