package node

import (
	"time"

	"github.com/nknorg/nkn/common"
)

const (
	HashCacheExpiration      = 3600 * time.Second
	HashCacheCleanupInterval = 60 * time.Second
)

type hashCache struct {
	cache common.Cache
}

func NewHashCache() *hashCache {
	return &hashCache{
		cache: common.NewGoCache(HashCacheExpiration, HashCacheCleanupInterval),
	}
}

func (hc *hashCache) ExistHash(hash common.Uint256) bool {
	err := hc.cache.Add(hash.ToArray(), struct{}{})
	return err != nil
}
