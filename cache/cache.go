package cache

import (
	"fmt"
	"strconv"

	"github.com/enverbisevac/gitlib/setting"
)

// Cache represents a caching interface
type Cache interface {
	// Put puts value into cache with key and expire time.
	Put(key string, val interface{}, timeout int64) error
	// Get gets cached value by given key.
	Get(key string) interface{}
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

// GetString returns the key value from cache with callback when no key exists in cache
func GetString(key string, getFunc func() (string, error)) (string, error) {
	if cache == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}
	if !cache.IsExist(key) {
		var (
			value string
			err   error
		)
		if value, err = getFunc(); err != nil {
			return value, err
		}
		err = cache.Put(key, value, setting.CacheService.TTLSeconds())
		if err != nil {
			return "", err
		}
	}
	value := cache.Get(key)
	if v, ok := value.(string); ok {
		return v, nil
	}
	if v, ok := value.(fmt.Stringer); ok {
		return v.String(), nil
	}
	return fmt.Sprintf("%s", cache.Get(key)), nil
}

// GetInt returns key value from cache with callback when no key exists in cache
func GetInt(key string, getFunc func() (int, error)) (int, error) {
	if cache == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}
	if !cache.IsExist(key) {
		var (
			value int
			err   error
		)
		if value, err = getFunc(); err != nil {
			return value, err
		}
		err = cache.Put(key, value, setting.CacheService.TTLSeconds())
		if err != nil {
			return 0, err
		}
	}
	switch value := cache.Get(key).(type) {
	case int:
		return value, nil
	case string:
		v, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return v, nil
	default:
		return 0, fmt.Errorf("Unsupported cached value type: %v", value)
	}
}

// GetInt64 returns key value from cache with callback when no key exists in cache
func GetInt64(key string, getFunc func() (int64, error)) (int64, error) {
	if cache == nil || setting.CacheService.TTL == 0 {
		return getFunc()
	}
	if !cache.IsExist(key) {
		var (
			value int64
			err   error
		)
		if value, err = getFunc(); err != nil {
			return value, err
		}
		err = cache.Put(key, value, setting.CacheService.TTLSeconds())
		if err != nil {
			return 0, err
		}
	}
	switch value := cache.Get(key).(type) {
	case int64:
		return value, nil
	case string:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
		return v, nil
	default:
		return 0, fmt.Errorf("Unsupported cached value type: %v", value)
	}
}

// Remove key from cache
func Remove(key string) {
	if cache == nil {
		return
	}
	_ = cache.Delete(key)
}
