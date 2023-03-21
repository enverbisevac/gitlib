package cache

import (
	"fmt"

	"github.com/enverbisevac/gitlib/setting"
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

var cache Cache

func Initialize(c Cache) {
	cache = c
}

// GetCache returns the currently configured cache
func GetCache() Cache {
	return cache
}

// Get returns the key value from cache with callback when no key exists in cache
func Get[T any](key string, getFunc func() (T, error)) (T, error) {
	var (
		empty T
	)

	if cache == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}

	if !cache.IsExist(key) {
		value, err := getFunc()
		if err != nil {
			return value, err
		}
		err = cache.Put(key, value, setting.CacheService.TTLSeconds())
		if err != nil {
			return empty, err
		}
	}
	value := cache.Get(key)
	if v, ok := value.(T); ok {
		return v, nil
	}
	return empty, fmt.Errorf("unsupported cached value type: %v", value)
}

// Remove key from cache
func Remove(key string) {
	if cache == nil {
		return
	}
	_ = cache.Delete(key)
}
