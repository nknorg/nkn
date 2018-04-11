package ising

import (
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"time"
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
	cap    int
	currentHeight uint32                //current block height
	cache  map[uint32]*BlockInfo // height and block mapping
	hashes map[Uint256]uint32    // for block fast searching
}

// When receive new block message from consensus layer, cache it.
func NewCache() *BlockCache {
	blockCache := &BlockCache{
		cap:    MaxCachedBlocks,
		currentHeight: 0,
		cache:  make(map[uint32]*BlockInfo),
		hashes: make(map[Uint256]uint32),
	}
	go blockCache.Cleanup()

	return blockCache
}

// BlockInCache returns whether the block has been cached.
func (p *BlockCache) BlockInCache(hash Uint256) bool {
	p.RLock()
	defer p.RUnlock()

	if _, ok := p.hashes[hash]; ok {
		return true
	}

	return false
}

// GetBlockFromCache returns block according to bloch hash passed in.
func (p *BlockCache) GetBlockFromCache(hash Uint256) *ledger.Block {
	p.RLock()
	defer p.RUnlock()

	if i, ok := p.hashes[hash]; !ok {
		return nil
	} else {
		if j, ok := p.cache[i]; !ok {
			return nil
		} else {
			return j.block
		}
	}
}

// GetCurrentBlockFromCache returns latest block in cache
func (p *BlockCache) GetCurrentBlockFromCache() *ledger.Block {
	p.RLock()
	defer p.RUnlock()

	if v, ok := p.cache[p.currentHeight]; ok {
		return v.block
	}

	return nil
}

// RemoveBlockFromCache return true if the block doesn't exist in cache.
func (p *BlockCache) RemoveBlockFromCache(hash Uint256) error {
	p.Lock()
	defer p.Unlock()

	if p.BlockInCache(hash) {
		h := p.hashes[hash]
		delete(p.cache, h)
		delete(p.hashes, hash)
		if p.currentHeight > 0 {
			p.currentHeight --
		}
	}

	return nil
}

// CachedBlockNum return the block number in cache
func (p *BlockCache) CachedBlockNum() int {
	p.RLock()
	defer p.RUnlock()

	return len(p.cache)
}

// AddBlockToCache returns nil if block already existed in cache
func (p *BlockCache) AddBlockToCache(block *ledger.Block) error {
	hash := block.Hash()
	if p.BlockInCache(hash) {
		return nil
	}
	// TODO FIFO cleanup, if cap space is not enough then
	// remove block from cache according to FIFO
	blockInfo := &BlockInfo{
		block:    block,
		lifetime: time.NewTimer(time.Hour),
	}
	p.Lock()
	defer p.Unlock()
	blockHeight := block.Header.Height
	p.cache[blockHeight] = blockInfo
	p.hashes[hash] = blockHeight
	p.currentHeight = blockHeight

	return nil
}

// Cleanup is a background routine used for cleaning up expired block in cache
func (p *BlockCache) Cleanup() {
	ticket := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticket.C:
			for _, blockInfo := range p.cache {
				select {
				case <-blockInfo.lifetime.C:
					p.RemoveBlockFromCache(blockInfo.block.Hash())
				}
			}
		}
	}
}
