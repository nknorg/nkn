package voting

import (
	"sync"

	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/util/log"
)

const (
	MaxCachedBlockHeight = 5
)

type BlockCache struct {
	sync.RWMutex
	cap    int
	cache  map[uint32][]*ledger.Block // height and block mapping
	hashes map[Uint256]uint32         // for block fast searching
}

// When receive new block message from consensus layer, cache it.
func NewBlockCache() *BlockCache {
	blockCache := &BlockCache{
		cap:    MaxCachedBlockHeight,
		cache:  make(map[uint32][]*ledger.Block),
		hashes: make(map[Uint256]uint32),
	}

	return blockCache
}

// BlockInCache returns whether the block has been cached.
func (bc *BlockCache) BlockInCache(hash Uint256, height uint32) bool {
	bc.RLock()
	defer bc.RUnlock()

	if blocks, ok := bc.cache[height]; ok {
		for _, b := range blocks {
			if hash.CompareTo(b.Hash()) == 0 {
				return true
			}
		}
	}

	return false
}

// GetBlockFromCache returns block according to block hash passed in.
func (bc *BlockCache) GetBlockFromCache(hash Uint256, height uint32) *ledger.Block {
	bc.RLock()
	defer bc.RUnlock()

	if _, ok := bc.hashes[hash]; !ok {
		return nil
	}
	for _, b := range bc.cache[height] {
		if hash.CompareTo(b.Hash()) == 0 {
			return b
		}
	}

	return nil
}

// GetBestBlockFromCache returns latest block in cache
func (bc *BlockCache) GetBestBlockFromCache(height uint32) *ledger.Block {
	bc.RLock()
	defer bc.RUnlock()

	if blocks, ok := bc.cache[height]; !ok {
		return nil
	} else {
		if len(blocks) == 0 {
			return nil
		}
		minBlock := blocks[0]
		minBlockHash := minBlock.Hash()
		for _, v := range blocks[1:] {
			if minBlockHash.CompareTo(v.Hash()) == 1 {
				minBlock = v
				minBlockHash = v.Hash()
			}
		}
		return minBlock
	}

	return nil
}

// GetBestBlockFromCache returns latest block in cache
func (bc *BlockCache) GetWorseBlockFromCache(height uint32) *ledger.Block {
	bc.RLock()
	defer bc.RUnlock()

	if blocks, ok := bc.cache[height]; !ok {
		return nil
	} else {
		if len(blocks) == 0 {
			return nil
		}
		minBlock := blocks[0]
		minBlockHash := minBlock.Hash()
		for _, v := range blocks[1:] {
			if minBlockHash.CompareTo(v.Hash()) == -1 {
				minBlock = v
				minBlockHash = v.Hash()
			}
		}
		return minBlock
	}

	return nil
}

// RemoveBlockFromCache return true if the block doesn't exist in cache.
func (bc *BlockCache) RemoveBlockFromCache(hash Uint256, height uint32) error {
	if !bc.BlockInCache(hash, height) {
		return nil
	}

	bc.Lock()
	defer bc.Unlock()

	delete(bc.hashes, hash)

	var blocks []*ledger.Block
	for k, v := range bc.cache[height] {
		if hash.CompareTo(v.Hash()) == 0 {
			blocks = append(blocks, bc.cache[height][:k]...)
			blocks = append(blocks, bc.cache[height][k+1:]...)
			break
		}
	}
	if blocks == nil {
		delete(bc.cache, height)
	} else {
		bc.cache[height] = blocks
	}
	return nil
}

// RemoveBlocksByHeight removes cached blocks according to the height.
func (bc *BlockCache) RemoveBlocksByHeight(height uint32) error {
	bc.Lock()
	defer bc.Unlock()

	if blocks, ok := bc.cache[height]; !ok {
		return nil
	} else {
		for _, block := range blocks {
			delete(bc.hashes, block.Hash())
		}
		delete(bc.cache, height)
	}

	return nil
}

// CachedBlockNum return the block number in cache
func (bc *BlockCache) CachedBlockNum() int {
	bc.RLock()
	defer bc.RUnlock()

	count := 0
	for _, v := range bc.cache {
		count += len(v)
	}

	return count
}

// CachedBlockHeight return cached block height
func (bc *BlockCache) CachedBlockHeight() int {
	bc.RLock()
	defer bc.RUnlock()

	return len(bc.cache)
}

// GetMinCachedHeight returns min block height cached
func (bc *BlockCache) GetMinCachedHeight() uint32 {
	bc.RLock()
	defer bc.RUnlock()

	var minHeight uint32
	for height := range bc.cache {
		minHeight = height
	}
	for height := range bc.cache {
		if height < minHeight {
			minHeight = height
		}
	}

	return minHeight
}

// AddBlockToCache returns nil if block already existed in cache
func (bc *BlockCache) AddBlockToCache(block *ledger.Block) error {
	hash := block.Hash()
	blockHeight := block.Header.Height
	if bc.BlockInCache(hash, blockHeight) {
		return nil
	}
	if bc.CachedBlockHeight() >= bc.cap {
		minHeight := bc.GetMinCachedHeight()
		bc.RemoveBlocksByHeight(minHeight)
	}
	bc.Lock()
	defer bc.Unlock()
	bc.cache[blockHeight] = append(bc.cache[blockHeight], block)
	bc.hashes[hash] = blockHeight

	return nil
}

func (bc *BlockCache) Dump(height uint32) {
	log.Infof("\t height: %d", height)
	for _, block := range bc.cache[height] {
		hash := block.Hash()
		log.Infof("\t\t hash: %s", BytesToHexString(hash.ToArray()))
	}
}
