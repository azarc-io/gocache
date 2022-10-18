package cache

import (
	"context"
	"errors"
	"github.com/eko/gocache/v3/store"
	mocksCache "github.com/eko/gocache/v3/test/mocks/cache"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func getMockCache[T any](t *testing.T) *mocksCache.MockSetterCacheInterface[T] {
	return mocksCache.NewMockSetterCacheInterface[T](gomock.NewController(t))
}

func TestNewStaleable(t *testing.T) {
	// Given
	ic := getMockCache[any](t)

	// When
	s := NewStaleable[any](ic, time.Second)

	// Then
	assert.IsType(t, new(StaleableCache[any]), s)
	assert.Equal(t, ic, s.cache)
}

func TestStaleCacheGet(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"

	ic.EXPECT().Get(ctx, cacheKey).Return(cacheValue, nil)

	// When
	s := NewStaleable[any](ic, time.Second)
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)
}

func TestStaleCacheGetWhenError(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	ic.EXPECT().Get(ctx, cacheKey).Return(nil, store.NotFound{})

	// When
	s := NewStaleable[any](ic, time.Second)
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, value)
	assert.ErrorIs(t, err, store.NotFound{})
}

func TestStaleCacheGetWithTTL(t *testing.T) {
	addCacheDuration := 1 * time.Second
	extendedExpiry := 5 * time.Second

	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, addCacheDuration+extendedExpiry, nil)

	// When
	s := NewStaleable[any](ic, extendedExpiry)
	value, ttl, err := s.GetWithTTL(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)
	assert.LessOrEqual(t, ttl.Milliseconds(), addCacheDuration.Milliseconds()+5)
	assert.GreaterOrEqual(t, ttl.Milliseconds(), addCacheDuration.Milliseconds()-5)
}

func TestStaleCacheGetWithTTLButStaleCache(t *testing.T) {
	addCacheDuration := -2 * time.Second // expired by 2s
	extendedExpiry := 5 * time.Second    // extended expiry of 5s
	remainingDur := extendedExpiry + addCacheDuration

	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, remainingDur, nil)

	// When
	s := NewStaleable[any](ic, extendedExpiry)
	value, ttl, err := s.GetWithTTL(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)
	assert.Equal(t, ttl, -2*time.Second) // note: negative number for remaining TTL indicates stale cache
}

func TestStaleCacheGetWithTTLWhenError(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, 0*time.Second, store.NotFound{})

	// When
	s := NewStaleable[any](ic, 1*time.Second)
	value, ttl, err := s.GetWithTTL(ctx, cacheKey)

	// Then
	assert.Nil(t, value)
	assert.ErrorIs(t, err, store.NotFound{})
	assert.Equal(t, 0*time.Second, ttl)
}

func TestStaleCacheSet(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()
	expiry := store.WithExpiration(1 * time.Second)

	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	var options []store.Option
	call := ic.EXPECT().Set(ctx, cacheKey, cacheValue, gomock.Any(), gomock.Any()).Return(nil)
	call.Do(func(ctx, cacheKey, cacheValue any, opts ...store.Option) {
		options = opts
	})

	// When
	s := NewStaleable[any](ic, 5*time.Second)
	err := s.Set(ctx, cacheKey, cacheValue, expiry)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, 2, len(options))
	opts := store.ApplyOptions(options...)
	assert.Equal(t, opts.Expiration(), 6*time.Second)
}

func TestStaleCacheDelete(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()
	cacheKey := "my-key"

	ic.EXPECT().Delete(ctx, cacheKey).Return(nil)

	// When
	s := NewStaleable[any](ic, time.Second)
	err := s.Delete(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
}

func TestStaleCacheInvalidate(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()
	tags := []string{"a23fdf987h2svc23", "jHG2372x38hf74"}

	var options []store.InvalidateOption
	ic.EXPECT().Invalidate(ctx, gomock.Any()).Return(nil).Do(func(ctx any, opts ...store.InvalidateOption) {
		options = opts
	})

	// When
	s := NewStaleable[any](ic, time.Second)
	err := s.Invalidate(ctx, store.WithInvalidateTags(tags))

	// Then
	assert.Nil(t, err)
	assert.Equal(t, 1, len(options))
	assert.Equal(t, tags, store.ApplyInvalidateOptions(options...).Tags())
}

func TestStaleCacheInvalidateWhenError(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()
	expectErr := errors.New("dummy")
	tags := []string{"a23fdf987h2svc23", "jHG2372x38hf74"}

	ic.EXPECT().Invalidate(ctx, gomock.Any()).Return(expectErr)

	// When
	s := NewStaleable[any](ic, time.Second)
	err := s.Invalidate(ctx, store.WithInvalidateTags(tags))

	// Then
	assert.Equal(t, expectErr, err)
}

func TestStaleCacheClear(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	ic.EXPECT().Clear(ctx)

	// When
	s := NewStaleable[any](ic, time.Second)
	err := s.Clear(ctx)

	// Then
	assert.Nil(t, err)
}

func TestStaleCacheGetType(t *testing.T) {
	// Given
	ic := getMockCache[any](t)

	// When - Then
	s := NewStaleable[any](ic, time.Second)
	assert.Equal(t, StaleableType, s.GetType())
}
