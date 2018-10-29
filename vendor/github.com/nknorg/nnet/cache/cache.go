package cache

// Cache is an anstract cache layer
type Cache interface {
	Add(key []byte, value interface{}) error
	Get(key []byte) (value interface{}, found bool)
	Set(key []byte, value interface{}) error
}
