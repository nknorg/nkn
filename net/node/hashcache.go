package node

import (
	"time"

	"github.com/nknorg/nkn/common"
)

const (
	HashCacheExpiration      = 60 * time.Second
	HashCacheCleanupInterval = 1 * time.Second
)

type hashCache struct {
	cache common.Cache
}

func NewHashCache() *hashCache {
	return &hashCache{
		cache: common.NewGoCache(HashCacheExpiration, HashCacheCleanupInterval),
	}
}

func (hc *hashCache) addHash(hash common.Uint256) {
	hc.cache.Set(hash.ToArray(), true)
}

func (hc *hashCache) ExistHash(hash common.Uint256) bool {
	_, ok := hc.cache.Get(hash.ToArray())
	hc.cache.Set(hash.ToArray(), true)
	return ok
}
