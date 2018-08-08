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

func (hc *hashCache) addHash(hash Uint256) {
	if len(hc.hashes) <= HashCacheCap {
		hc.hashes = append(hc.hashes, hash)
	} else {
		delete(hc.list, hc.hashes[0])
		hc.hashes = append(hc.hashes[1:], hash)
	}
	hc.list[hash] = struct{}{}
}

func (hc *hashCache) ExistHash(hash Uint256) bool {
	hc.Lock()
	defer hc.Unlock()

	if _, ok := hc.list[hash]; ok {
		return true
	}
	hc.addHash(hash)

	return false
}
