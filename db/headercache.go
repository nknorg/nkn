package db

import (
	"fmt"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/core/ledger"
)

type HeaderCache struct {
	mu                 sync.RWMutex
	headerIndex        map[uint32]common.Uint256
	headerCache        map[common.Uint256]*ledger.Header
	currentCacheHeight uint32
}

func NewHeaderCache() *HeaderCache {
	return &HeaderCache{
		headerIndex:        map[uint32]common.Uint256{},
		headerCache:        map[common.Uint256]*ledger.Header{},
		currentCacheHeight: 0,
	}
}

func (hc *HeaderCache) AddHeaderToCache(header *ledger.Header) {
	hash := header.Hash()

	hc.mu.Lock()
	hc.headerCache[hash] = header
	hc.headerIndex[header.Height] = hash
	if hc.currentCacheHeight < header.Height {
		hc.currentCacheHeight = header.Height
	}
	hc.mu.Unlock()
}

func (hc *HeaderCache) RemoveCachedHeader(stopHeight uint32) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for height, hash := range hc.headerIndex {
		if height < stopHeight {
			delete(hc.headerIndex, height)
			delete(hc.headerCache, hash)
		}
	}
	if hc.currentCacheHeight <= stopHeight {
		hc.currentCacheHeight = 0
	}
}

func (hc *HeaderCache) GetCachedHeader(hash common.Uint256) *ledger.Header {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if _, ok := hc.headerCache[hash]; ok {
		return hc.headerCache[hash]
	}

	return nil
}

func (hc *HeaderCache) GetCurrentCacheHeaderHash() common.Uint256 {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.headerIndex[hc.currentCacheHeight]

}

func (hc *HeaderCache) GetCurrentCachedHeight() uint32 {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.currentCacheHeight
}

func (hc *HeaderCache) GetCachedHeaderHashByHeight(height uint32) common.Uint256 {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.headerIndex[height]
}

func (hc *HeaderCache) Dump() {
	fmt.Println("headerIndex:")
	for height, hash := range hc.headerIndex {
		fmt.Println(height, hash.ToHexString())
	}

	fmt.Println("headerCache")
	for hash, header := range hc.headerCache {
		fmt.Println(hash.ToHexString(), header)
	}
	fmt.Println("currentCacheHeight:", hc.currentCacheHeight)

}
