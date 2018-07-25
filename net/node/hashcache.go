package node

import (
	"sync"

	. "github.com/nknorg/nkn/common"
)

const (
	HashCacheCap = 1000
)

type hashCache struct {
	sync.RWMutex
	cap    int
	list   map[Uint256]struct{}
	hashes []Uint256
}

func NewHashCache(cap int) *hashCache {
	return &hashCache{
		cap:  cap,
		list: make(map[Uint256]struct{}),
	}
}

func (hc *hashCache) AddHash(hash Uint256) {
	hc.Lock()
	defer hc.Unlock()

	if len(hc.hashes) <= HashCacheCap {
		hc.hashes = append(hc.hashes, hash)
	} else {
		delete(hc.list, hc.hashes[0])
		hc.hashes = append(hc.hashes[1:], hash)
	}
	hc.list[hash] = struct{}{}
}

func (hc *hashCache) ExistHash(hash Uint256) bool {
	hc.RLock()
	defer hc.RUnlock()

	if _, ok := hc.list[hash]; ok {
		return true
	}

	return false
}
