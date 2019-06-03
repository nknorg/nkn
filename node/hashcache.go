package node

import (
	"time"

	"github.com/nknorg/nkn/common"
)

const (
	hashCacheExpiration      = 3600 * time.Second
	hashCacheCleanupInterval = 60 * time.Second
)

type hashCache struct {
	common.Cache
}

func newHashCache() *hashCache {
	return &hashCache{
		Cache: common.NewGoCache(hashCacheExpiration, hashCacheCleanupInterval),
	}
}

func (hc *hashCache) ExistHash(hash common.Uint256) bool {
	err := hc.Add(hash.ToArray(), struct{}{})
	return err != nil
}
