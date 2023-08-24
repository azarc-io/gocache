package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/eko/gocache/v3/store"
	mocksCache "github.com/eko/gocache/v3/test/mocks/cache"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type TTLOptionMatcher struct {
	TTL time.Duration
}

func (o *TTLOptionMatcher) Matches(x interface{}) bool {
	if opt, ok := x.(store.Option); ok {
		opts := store.ApplyOptions(opt)
		return o.TTL == opts.Expiration()
	}
	return false
}

func (o *TTLOptionMatcher) String() string {
	return fmt.Sprintf("ttl: %d", o.TTL)
}

func getMockCache[T any](t *testing.T) *mocksCache.MockSetterCacheInterface[T] {
	return mocksCache.NewMockSetterCacheInterface[T](gomock.NewController(t))
}

func TestNewStaleable(t *testing.T) {
	// Given
	ic := getMockCache[any](t)

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))

	// Then
	assert.IsType(t, new(StaleableCache[any]), s)
	assert.Equal(t, ic, s.cache)
}

func TestStaleCacheGetWithoutRefreshFunction(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"

	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, time.Minute, nil)

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)
}

func TestStaleCacheGetWithRefreshFunction(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	updatedCacheValue := "updated-cache-value"
	ttl := time.Second
	staleTTL := time.Minute

	loadFunc := func(_ context.Context, key any) (any, error) {
		return updatedCacheValue, nil
	}

	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, 2*time.Second, nil)
	ic.EXPECT().Set(ctx, cacheKey, updatedCacheValue, gomock.Any())

	// When
	s := NewStaleable[any](ic, WithTTL[any](ttl), WithMaxStaleCacheTTL[any](staleTTL), WithStaleCacheLoadFunction[any](loadFunc))
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)

	// wait for the background refresh to finish
	time.Sleep(1 * time.Millisecond)
}

func TestStaleCacheGetWithRefreshFunctionAndValidCache(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	updatedCacheValue := "updated-cache-value"

	loadFunc := func(_ context.Context, key any) (any, error) {
		return updatedCacheValue, nil
	}

	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, time.Minute, nil)

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second), WithStaleCacheLoadFunction[any](loadFunc))
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)

	// wait for the background refresh to finish
	time.Sleep(1 * time.Millisecond)
}

func TestStaleCacheGetWhenError(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, time.Duration(0), store.NotFound{})

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](extendedExpiry))
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](extendedExpiry))
	value, ttl, err := s.GetWithTTL(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, cacheValue, value)
	assert.Equal(t, ttl, -2*time.Second) // note: negative number for remaining TTL indicates stale cache
}

func TestStaleCacheGetWithNegativeTTL(t *testing.T) {
	ttlValue := time.Second
	staleTTLValue := time.Minute

	// Given
	ic := getMockCache[any](t)
	ctx := context.Background()

	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	updatedCacheValue := "updated-cache-value"
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, -time.Second, nil)
	ic.EXPECT().Set(ctx, cacheKey, updatedCacheValue, &TTLOptionMatcher{TTL: ttlValue + staleTTLValue}).Return(nil)

	loadFunc := func(_ context.Context, key any) (any, error) {
		return updatedCacheValue, nil
	}
	// When
	s := NewStaleable[any](ic,
		WithTTL[any](ttlValue),
		WithMaxStaleCacheTTL[any](staleTTLValue),
		WithStaleCacheLoadFunction[any](loadFunc),
	)
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, err)
	assert.Equal(t, updatedCacheValue, value)
}

func TestStaleCacheGetWithTTLWhenError(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, 0*time.Second, store.NotFound{})

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](1*time.Second))
	value, ttl, err := s.GetWithTTL(ctx, cacheKey)

	// Then
	assert.Nil(t, value)
	assert.ErrorIs(t, err, store.NotFound{})
	assert.Equal(t, 0*time.Second, ttl)
}

func TestStaleCacheGetWithNoCacheEntryAndNoLoader(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, 0*time.Second, &store.NotFound{})

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](3*time.Second))
	value, err := s.Get(ctx, cacheKey)

	// Then
	assert.Nil(t, value)
	assert.NoError(t, err)
}

func TestStaleCacheGetWithParallelRequests(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"
	cacheValue := "my-cache-value"
	updatedCacheValue := "updated-cache-value"
	ttlValue := time.Second
	staleTTLValue := time.Minute

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(cacheValue, 2*time.Second, nil)
	ic.EXPECT().Set(ctx, cacheKey, updatedCacheValue, &TTLOptionMatcher{TTL: ttlValue + staleTTLValue}).Times(1)

	// When
	s := NewStaleable[any](ic,
		WithTTL[any](ttlValue),
		WithMaxStaleCacheTTL[any](staleTTLValue),
		WithStaleCacheLoadFunction[any](func(_ context.Context, key any) (any, error) {
			time.Sleep(time.Second)
			return updatedCacheValue, nil
		}))
	for i := 0; i < 3; i++ {
		go func() {
			value, err := s.Get(ctx, cacheKey)

			assert.Equal(t, cacheValue, value)
			assert.NoError(t, err)
		}()
	}

	time.Sleep(2 * time.Second)
}

func TestStaleCacheGetWithParallelRequestsWithNoCacheEntry(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"
	cacheValue := "my-cache-value"

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, 0*time.Second, &store.NotFound{})
	ic.EXPECT().Set(ctx, cacheKey, cacheValue, gomock.Any()).Times(1)

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](3*time.Second), WithStaleCacheLoadFunction[any](func(_ context.Context, key any) (any, error) {
		time.Sleep(time.Second)
		return cacheValue, nil
	}))
	for i := 0; i < 3; i++ {
		go func() {
			value, err := s.Get(ctx, cacheKey)

			assert.Equal(t, cacheValue, value)
			assert.NoError(t, err)
		}()
	}

	time.Sleep(2 * time.Second)
}

func TestStaleCacheGetWithParallelRequestsWithLoaderError(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"
	cacheError := "my-cache-error"

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, 0*time.Second, &store.NotFound{})

	// When
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](3*time.Second), WithStaleCacheLoadFunction[any](func(_ context.Context, key any) (any, error) {
		time.Sleep(time.Second)
		return nil, errors.New(cacheError)
	}))
	for i := 0; i < 3; i++ {
		go func() {
			value, err := s.Get(ctx, cacheKey)

			assert.Nil(t, value)
			assert.EqualError(t, err, cacheError)
		}()
	}

	time.Sleep(2 * time.Second)
}

func TestStaleCacheGetWithParallelRequestsWithShouldNotCachePredicate(t *testing.T) {
	// Given
	ic := getMockCache[any](t)
	cacheKey := "my-key"
	cacheValue := "my-cache-value"

	ctx := context.Background()
	ic.EXPECT().GetWithTTL(ctx, cacheKey).Return(nil, 0*time.Second, &store.NotFound{})

	// When
	s := NewStaleable[any](ic,
		WithMaxStaleCacheTTL[any](3*time.Second),
		WithStaleCacheLoadFunction[any](func(_ context.Context, key any) (any, error) {
			time.Sleep(time.Second)
			return cacheValue, nil
		}),
		WithStaleCachePredicate(func(_ any, response any) bool {
			return false
		}))
	for i := 0; i < 3; i++ {
		go func() {
			value, err := s.Get(ctx, cacheKey)

			assert.Equal(t, cacheValue, value)
			assert.NoError(t, err)
		}()
	}

	time.Sleep(2 * time.Second)
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](5*time.Second))
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
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
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
	err := s.Clear(ctx)

	// Then
	assert.Nil(t, err)
}

func TestStaleCacheGetType(t *testing.T) {
	// Given
	ic := getMockCache[any](t)

	// When - Then
	s := NewStaleable[any](ic, WithMaxStaleCacheTTL[any](time.Second))
	assert.Equal(t, StaleableType, s.GetType())
}
