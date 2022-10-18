package cache

import (
	"context"
	"github.com/eko/gocache/v3/store"
	"time"
)

const (
	// StaleableType represents the staleable cache type as a string value
	StaleableType = "staleable"
)

// StaleableCache is a wrapper cache which extends expiry of cached items to allow stale cache retrieval
type StaleableCache[T any] struct {
	cache      SetterCacheInterface[T]
	minimumTTL time.Duration
}

// NewStaleable creates a new wrapper cache StaleableCache instance
func NewStaleable[T any](underlyingCache SetterCacheInterface[T], maxStaleCacheTTL time.Duration) *StaleableCache[T] {
	return &StaleableCache[T]{
		cache:      underlyingCache,
		minimumTTL: maxStaleCacheTTL,
	}
}

// Get returns data stored from a given key
func (s *StaleableCache[T]) Get(ctx context.Context, key any) (T, error) {
	var err error

	object, err := s.cache.Get(ctx, key)
	if err == nil {
		return object, err
	}
	//TODO: check TTL and refresh record if needed

	return object, err
}

// GetWithTTL returns data stored from a given key and its corresponding TTL.
//  when negative TTL is returned, this means that the original cache TTL expired and should be refreshed
func (s *StaleableCache[T]) GetWithTTL(ctx context.Context, key any) (T, time.Duration, error) {
	value, storeCacheDuration, err := s.cache.GetWithTTL(ctx, key)
	if err != nil {
		return *new(T), 0, err
	}

	storeCacheDuration = storeCacheDuration - s.minimumTTL
	return value, storeCacheDuration, nil
}

// Set data in the underlying cache with expiry
func (s *StaleableCache[T]) Set(ctx context.Context, key any, value T, options ...store.Option) error {
	opts := store.ApplyOptions(options...)
	var expiration time.Duration
	if opts != nil {
		expiration = opts.Expiration()
	}

	options = append(options, store.WithExpiration(s.minimumTTL+expiration))
	return s.cache.Set(ctx, key, value, options...)
}

// Delete removes data in underlying cache for given key identifier
func (s *StaleableCache[T]) Delete(ctx context.Context, key any) error {
	return s.cache.Delete(ctx, key)
}

// Invalidate invalidates some cache data in underlying cache for given options
func (s *StaleableCache[T]) Invalidate(ctx context.Context, options ...store.InvalidateOption) error {
	return s.cache.Invalidate(ctx, options...)
}

// GetType returns the store type
func (s *StaleableCache[T]) GetType() string {
	return StaleableType
}

// Clear resets all data in the store
func (s *StaleableCache[T]) Clear(ctx context.Context) error {
	return s.cache.Clear(ctx)
}
