package ising

import (
	"sync"

	. "nkn-core/common"
	"nkn-core/core/ledger"
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
	cap   int
	cache map[Uint256]*BlockInfo
}

// When receive new block message from consensus layer, cache it.
func NewCache() *BlockCache {
	blockCache := &BlockCache{
		cap:   MaxCachedBlocks,
		cache: make(map[Uint256]*BlockInfo),
	}
	go blockCache.Cleanup()

	return blockCache
}

// BlockInCache returns whether the block has been cached.
func (p *BlockCache) BlockInCache(hash Uint256) bool {
	p.RLock()
	defer p.RUnlock()

	if _, ok := p.cache[hash]; ok {
		return true
	}

	return false
}

// RemoveBlockFromCache return true if the block doesn't exist in cache.
func (p *BlockCache) RemoveBlockFromCache(hash Uint256) error {
	p.Lock()
	defer p.Unlock()
	if p.BlockInCache(hash) {
		delete(p.cache, hash)
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
	p.Lock()
	defer p.Unlock()
	blockInfo := &BlockInfo{
		block:    block,
		lifetime: time.NewTimer(time.Hour),
	}
	p.cache[hash] = blockInfo

	return nil
}

// Cleanup is a background routine used for cleaning up expired block in cache
func (p *BlockCache) Cleanup() {
	ticket := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticket.C:
			for blockHash, blockInfo := range p.cache {
				select {
				case <-blockInfo.lifetime.C:
					p.RemoveBlockFromCache(blockHash)
				}
			}
		}
	}
}
