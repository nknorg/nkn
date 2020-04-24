package common

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Cache is an anstract cache layer
type Cache interface {
	Add(key []byte, value interface{}) error
	Get(key []byte) (value interface{}, ok bool)
	Set(key []byte, value interface{}) error
	SetWithExpiration(key []byte, value interface{}, expiration time.Duration) error
	Delete(key []byte) error
}

// GoCache is the caching layer implemented by go-cache.
type GoCache struct {
	*gocache.Cache
}

// NewGoCache creates a go-cache cache with a given default expiration duration
// and cleanup interval.
func NewGoCache(defaultExpiration, cleanupInterval time.Duration) *GoCache {
	return &GoCache{
		Cache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

// Add adds an item to the cache only if an item doesn't already exist for the
// given key, or if the existing item has expired. Returns an error otherwise.
func (gc *GoCache) Add(key []byte, value interface{}) error {
	return gc.Cache.Add(byteKeyToStringKey(key), value, gocache.DefaultExpiration)
}

// Get gets an item from the cache. Returns the item or nil, and a bool
// indicating whether the key was found.
func (gc *GoCache) Get(key []byte) (interface{}, bool) {
	return gc.Cache.Get(byteKeyToStringKey(key))
}

// Set adds an item to the cache, replacing any existing item, using the default
// expiration.
func (gc *GoCache) Set(key []byte, value interface{}) error {
	gc.Cache.SetDefault(byteKeyToStringKey(key), value)
	return nil
}

// SetWithExpiration adds an item to the cache, replacing any existing item,
// with a given expiration.
func (gc *GoCache) SetWithExpiration(key []byte, value interface{}, expiration time.Duration) error {
	gc.Cache.Set(byteKeyToStringKey(key), value, expiration)
	return nil
}

// Delete deletes an item from the cache. Does nothing if the key is not in the
// cache.
func (gc *GoCache) Delete(key []byte) error {
	gc.Cache.Delete(byteKeyToStringKey(key))
	return nil
}

func byteKeyToStringKey(byteKey []byte) string {
	return string(byteKey)
}
