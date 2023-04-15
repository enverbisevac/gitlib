package git

import (
	"fmt"
)

// Cache represents a caching interface
type Cache interface {
	// Put puts value into cache with key and expire time.
	Put(key string, val any, timeout int64) error
	// Get gets cached value by given key.
	Get(key string) any
	// IsExist checks if key exists
	IsExist(key string) bool
	Delete(key string) error
}

var lcache Cache

func Initialize(c Cache) {
	lcache = c
}

// GetCache returns the currently configured cache
func GetCache() Cache {
	return lcache
}

// Get returns the key value from cache with callback when no key exists in cache
func Get[T any](key string, getFunc func() (T, error)) (T, error) {
	var (
		empty T
	)

	if lcache == nil || CacheService.Cache.TTL == 0 {
		return getFunc()
	}

	if !lcache.IsExist(key) {
		value, err := getFunc()
		if err != nil {
			return value, err
		}
		err = lcache.Put(key, value, CacheService.Cache.TTL.Milliseconds())
		if err != nil {
			return empty, err
		}
	}
	value := lcache.Get(key)
	if v, ok := value.(T); ok {
		return v, nil
	}
	return empty, fmt.Errorf("unsupported cached value type: %v", value)
}

// Remove key from cache
func Remove(key string) {
	if lcache == nil {
		return
	}
	_ = lcache.Delete(key)
}
