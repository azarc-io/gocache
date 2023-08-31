package cache

import (
	"context"
	"fmt"
	"github.com/eko/gocache/v3/store"
	"sync"
	"time"
)

const (
	// StaleableType represents the staleable cache type as a string value
	StaleableType = "staleable"
)

type ShouldCachePredicate[T any] func(key any, value T) bool

type mapEntry[T any] struct {
	lockChannel chan bool
	value       T
	err         error
}

// StaleableCache is a wrapper cache which extends expiry of cached items to allow stale cache retrieval
type StaleableCache[T any] struct {
	cache       SetterCacheInterface[T]
	minimumTTL  time.Duration
	refreshTTL  time.Duration
	loadFunc    LoadFunction[T]
	shouldCache ShouldCachePredicate[T]

	inprogressMap sync.Map
}

type StaleableCacheOption[T any] func(cache *StaleableCache[T])

func WithTTL[T any](ttl time.Duration) StaleableCacheOption[T] {
	return func(cache *StaleableCache[T]) {
		cache.refreshTTL = ttl
	}
}

func WithMaxStaleCacheTTL[T any](ttl time.Duration) StaleableCacheOption[T] {
	return func(cache *StaleableCache[T]) {
		cache.minimumTTL = ttl
	}
}

func WithStaleCacheLoadFunction[T any](f LoadFunction[T]) StaleableCacheOption[T] {
	return func(cache *StaleableCache[T]) {
		cache.loadFunc = f
	}
}

func WithStaleCachePredicate[T any](f ShouldCachePredicate[T]) StaleableCacheOption[T] {
	return func(cache *StaleableCache[T]) {
		cache.shouldCache = f
	}
}

// NewStaleable creates a new wrapper cache StaleableCache instance
func NewStaleable[T any](underlyingCache SetterCacheInterface[T], opts ...StaleableCacheOption[T]) *StaleableCache[T] {
	staleableCache := &StaleableCache[T]{
		cache: underlyingCache,
	}
	for _, opt := range opts {
		opt(staleableCache)
	}
	return staleableCache
}

// Get returns data stored from a given key. If it's ttl is negative, refreshes the value in background
func (s *StaleableCache[T]) Get(ctx context.Context, key any) (T, error) {
	stringKey := getCacheKey(key)
	entry, inProgress := s.inprogressMap.LoadOrStore(stringKey, &mapEntry[T]{lockChannel: make(chan bool)})
	mEntry := entry.(*mapEntry[T])
	if inProgress {
		<-mEntry.lockChannel
		return mEntry.value, mEntry.err
	}

	object, ttl, err := s.GetWithTTL(ctx, key)
	mEntry.value = object
	mEntry.err = err
	if err != nil {
		if _, ok := err.(*store.NotFound); ok {
			// record does not exist, need to load it synchronously
			mEntry.value, mEntry.err = s.loadAndStore(ctx, key)
		}
		s.inprogressMap.Delete(stringKey)
	} else if ttl+s.minimumTTL < 0 {
		// record was expired but not removed automatically from the store
		mEntry.value, mEntry.err = s.loadAndStore(ctx, key)
		s.inprogressMap.Delete(stringKey)
	} else if ttl < 0 {
		// record is staled, need to refresh in background
		go func() {
			_, _ = s.loadAndStore(context.Background(), key)
			s.inprogressMap.Delete(stringKey)
		}()
	} else {
		s.inprogressMap.Delete(stringKey)
	}
	// close lockChannel to signal all other parallel calls they can read the result
	close(mEntry.lockChannel)

	return mEntry.value, mEntry.err
}

// loadAndStore calls loadFunc and stores result in cache
//
// If there is no loadFunc, don't do anything
//
// If loadFunc returns error, don't store it in cache, just return it
//
// If shouldCache predicate exists and it returns false, don't store anything in cache, just return the result
func (s *StaleableCache[T]) loadAndStore(ctx context.Context, key any) (T, error) {
	if s.loadFunc == nil {
		return *new(T), nil
	}

	res, err := s.loadFunc(ctx, key)
	if err != nil {
		return res, err
	}
	if s.shouldCache != nil && !s.shouldCache(key, res) {
		return res, err
	}
	_ = s.Set(ctx, key, res)

	return res, err
}

// GetWithTTL returns data stored from a given key and its corresponding TTL.
// when negative TTL is returned, this means that the original cache TTL expired and should be refreshed
func (s *StaleableCache[T]) GetWithTTL(ctx context.Context, key any) (T, time.Duration, error) {
	value, storeCacheDuration, err := s.cache.GetWithTTL(ctx, key)
	fmt.Println("go staleableCache: GetWithTTL: cachedValue: ", value)
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
	if expiration == 0 {
		expiration = s.refreshTTL
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
