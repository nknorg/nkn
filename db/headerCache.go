package db

import (
	"errors"
	"fmt"
	"sync"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/types"
)

type HeaderCache struct {
	mu                 sync.RWMutex
	headerIndex        map[uint32]common.Uint256
	headerCache        map[common.Uint256]types.Header
	currentCacheHeight uint32
}

func NewHeaderCache() *HeaderCache {
	return &HeaderCache{
		headerIndex:        map[uint32]common.Uint256{},
		headerCache:        map[common.Uint256]types.Header{},
		currentCacheHeight: 0,
	}
}

func (hc *HeaderCache) AddHeaderToCache(header *types.Header) {
	hc.mu.Lock()
	hash := header.Hash()
	hc.headerCache[hash] = *header
	hc.headerIndex[header.UnsignedHeader.Height] = hash
	if hc.currentCacheHeight < header.UnsignedHeader.Height {
		hc.currentCacheHeight = header.UnsignedHeader.Height
	}
	hc.mu.Unlock()
}

func (hc *HeaderCache) RemoveCachedHeader(stopHeight uint32) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.currentCacheHeight <= stopHeight {
		return
	}
	for hash, header := range hc.headerCache {
		if header.UnsignedHeader.Height < stopHeight {
			delete(hc.headerIndex, header.UnsignedHeader.Height)
			delete(hc.headerCache, hash)
		}
	}
}

func (hc *HeaderCache) GetCachedHeader(hash common.Uint256) (*types.Header, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if header, ok := hc.headerCache[hash]; !ok {
		return nil, errors.New("no header in cache")
	} else {
		return &header, nil
	}
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
