package store_wrappers

import (
	"context"
	"time"

	"github.com/eko/gocache/v3/store"
)

const (
	// StaleCacheWrapper represents the storage type as a string value
	StaleCacheWrapper = "stale-cache-wrapper"
)

// StaleCacheStore is a wrapper store which extends expiry of cached items to allow stale cache retrieval
type StaleCacheStore struct {
	cache      store.StoreInterface
	minimumTTL time.Duration
}

// NewStaleCache creates a new wrapper store StaleCache instance
func NewStaleCache(underlyingCache store.StoreInterface, maxStaleCacheTTL time.Duration) *StaleCacheStore {
	return &StaleCacheStore{
		cache:      underlyingCache,
		minimumTTL: maxStaleCacheTTL,
	}
}

// Get returns data stored from a given key
func (s *StaleCacheStore) Get(ctx context.Context, key any) (any, error) {
	return s.cache.Get(ctx, key)
}

// GetWithTTL returns data stored from a given key and its corresponding TTL.
//  when negative TTL is returned, this means that the original cache TTL expired and should be refreshed
func (s *StaleCacheStore) GetWithTTL(ctx context.Context, key any) (any, time.Duration, error) {
	value, storeCacheDuration, err := s.cache.GetWithTTL(ctx, key)
	if err != nil {
		return nil, 0, err
	}

	storeCacheDuration = storeCacheDuration - s.minimumTTL
	return value, storeCacheDuration, nil
}

// Set data in the underlying cache with expiry
func (s *StaleCacheStore) Set(ctx context.Context, key any, value any, options ...store.Option) error {
	opts := store.ApplyOptions(options...)
	var expiration time.Duration
	if opts != nil {
		expiration = opts.Expiration()
	}

	options = append(options, store.WithExpiration(s.minimumTTL+expiration))
	return s.cache.Set(ctx, key, value, options...)
}

// Delete removes data in underlying cache for given key identifier
func (s *StaleCacheStore) Delete(ctx context.Context, key any) error {
	return s.cache.Delete(ctx, key)
}

// Invalidate invalidates some cache data in underlying cache for given options
func (s *StaleCacheStore) Invalidate(ctx context.Context, options ...store.InvalidateOption) error {
	return s.cache.Invalidate(ctx, options...)
}

// GetType returns the store type
func (s *StaleCacheStore) GetType() string {
	return StaleCacheWrapper
}

// Clear resets all data in the store
func (s *StaleCacheStore) Clear(ctx context.Context) error {
	return s.cache.Clear(ctx)
}
