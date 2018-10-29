package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// GoCache is the caching layer implemented by go-cache.
type GoCache struct {
	cache *gocache.Cache
}

// NewGoCache creates a go-cache cache with a given default expiration duration
// and cleanup interval.
func NewGoCache(defaultExpiration, cleanupInterval time.Duration) *GoCache {
	return &GoCache{
		cache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

func (gc *GoCache) byteKeyToStringKey(byteKey []byte) string {
	return string(byteKey)
}

// Add adds an item to the cache only if an item doesn't already exist for the
// given key, or if the existing item has expired. Returns an error otherwise.
func (gc *GoCache) Add(key []byte, value interface{}) error {
	return gc.cache.Add(gc.byteKeyToStringKey(key), value, 0)
}

// Get gets an item from the cache. Returns the item or nil, and a bool
// indicating whether the key was found.
func (gc *GoCache) Get(key []byte) (interface{}, bool) {
	return gc.cache.Get(gc.byteKeyToStringKey(key))
}

// Set adds an item to the cache, replacing any existing item, using the default
// expiration.
func (gc *GoCache) Set(key []byte, value interface{}) error {
	gc.cache.SetDefault(gc.byteKeyToStringKey(key), value)
	return nil
}
