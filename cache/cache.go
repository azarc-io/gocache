package cache

import (
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"reflect"
	"time"

	"github.com/eko/gocache/v3/codec"
	"github.com/eko/gocache/v3/store"
)

const (
	// CacheType represents the cache type as a string value
	CacheType = "cache"
)

// Cache represents the configuration needed by a cache
type Cache[T any] struct {
	codec codec.CodecInterface
}

// New instantiates a new cache entry
func New[T any](store store.StoreInterface) *Cache[T] {
	return &Cache[T]{
		codec: codec.New(store),
	}
}

// Get returns the object stored in cache if it exists
func (c *Cache[T]) Get(ctx context.Context, key any) (T, error) {
	cacheKey := getCacheKey(key)

	value, err := c.codec.Get(ctx, cacheKey)
	if err != nil {
		return *new(T), err
	}

	return handleReturnValue[T](value), nil
}

// GetWithTTL returns the object stored in cache and its corresponding TTL
func (c *Cache[T]) GetWithTTL(ctx context.Context, key any) (T, time.Duration, error) {
	cacheKey := getCacheKey(key)

	value, duration, err := c.codec.GetWithTTL(ctx, cacheKey)
	if err != nil {
		return *new(T), duration, err
	}

	return handleReturnValue[T](value), duration, nil
}

// Set populates the cache item using the given key
func (c *Cache[T]) Set(ctx context.Context, key any, object T, options ...store.Option) error {
	cacheKey := getCacheKey(key)
	return c.codec.Set(ctx, cacheKey, object, options...)
}

// Delete removes the cache item using the given key
func (c *Cache[T]) Delete(ctx context.Context, key any) error {
	cacheKey := getCacheKey(key)
	return c.codec.Delete(ctx, cacheKey)
}

// Invalidate invalidates cache item from given options
func (c *Cache[T]) Invalidate(ctx context.Context, options ...store.InvalidateOption) error {
	return c.codec.Invalidate(ctx, options...)
}

// Clear resets all cache data
func (c *Cache[T]) Clear(ctx context.Context) error {
	return c.codec.Clear(ctx)
}

// GetCodec returns the current codec
func (c *Cache[T]) GetCodec() codec.CodecInterface {
	return c.codec
}

// GetType returns the cache type
func (c *Cache[T]) GetType() string {
	return CacheType
}

// getCacheKey returns the cache key for the given key object by returning
// the key if type is string or by computing a checksum of key structure
// if its type is other than string
func getCacheKey(key any) string {
	switch v := key.(type) {
	case string:
		return v
	case CacheKeyGenerator:
		return v.GetCacheKey()
	default:
		return checksum(key)
	}
}

// checksum hashes a given object into a string
func checksum(object any) string {
	digester := crypto.MD5.New()
	fmt.Fprint(digester, reflect.TypeOf(object))
	fmt.Fprint(digester, object)
	hash := digester.Sum(nil)

	return fmt.Sprintf("%x", hash)
}

func handleReturnValue[T any](value any) T {
	if v, ok := value.(T); ok {
		return v
	} else {
		// in case we have []byte in store, it is returned as base64 string but we expect []byte,
		// we need to decode the base64 string to []byte
		if valStr, valOk := value.(string); valOk {
			if _, tOk := any(*new(T)).([]byte); tOk {
				decodedValue, err := base64.StdEncoding.DecodeString(valStr)
				if err != nil {
					// return the string value as []byte if it's not a base64 string
					return any([]byte(valStr)).(T)
				}
				return any(decodedValue).(T)
			}
		}
	}
	return *new(T)
}
