package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, func() {
		client.Close()
		mr.Close()
	}
}

func TestSlidingWindowRateLimiter(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	window := 100 * time.Millisecond
	limit := 3
	limiter := NewRateLimiter(client, limit, window)
	key := "test-client"
	ctx := context.Background()

	t.Run("within limit", func(t *testing.T) {
		client.FlushAll(ctx)

		for i := range limit {
			result, err := limiter.Allow(ctx, key)
			assert.NoError(t, err)
			assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
			assert.Equal(t, int64(limit-i-1), result.Remaining)
			assert.Equal(t, limit, result.Limit)
		}
	})

	t.Run("exceeding limit", func(t *testing.T) {
		result, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.False(t, result.Allowed, "Request should be denied when exceeding limit")
		assert.Equal(t, int64(0), result.Remaining)
	})

	t.Run("sliding window behavior", func(t *testing.T) {
		client.FlushAll(ctx)

		for i := range limit {
			result, err := limiter.Allow(ctx, key)
			require.NoError(t, err)
			require.True(t, result.Allowed, "Request %d should be allowed", i+1)
		}

		time.Sleep(window / 2)

		result, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.False(t, result.Allowed, "Should still be limited before window expires")

		time.Sleep(window)

		result, err = limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, result.Allowed, "Should allow requests after window expires")
	})

	t.Run("multiple keys", func(t *testing.T) {
		client.FlushAll(ctx)

		key1 := "client1"
		key2 := "client2"

		for i := range limit {
			result, err := limiter.Allow(ctx, key1)
			require.NoError(t, err)
			require.True(t, result.Allowed, "Request %d for key1 should be allowed", i+1)
		}

		result, err := limiter.Allow(ctx, key2)
		assert.NoError(t, err)
		assert.True(t, result.Allowed, "Different keys should have separate limits")
	})

	t.Run("result contains correct metadata", func(t *testing.T) {
		client.FlushAll(ctx)

		result, err := limiter.Allow(ctx, key)
		require.NoError(t, err)

		assert.True(t, result.Allowed)
		assert.Equal(t, limit, result.Limit)
		assert.Equal(t, int64(limit-1), result.Remaining)
		assert.True(t, result.ResetAt.After(time.Now()))
	})

	t.Run("error handling", func(t *testing.T) {
		client.Close()

		result, err := limiter.Allow(ctx, key)
		assert.Error(t, err, "Should return error when Redis is unavailable")
		assert.False(t, result.Allowed, "Should deny requests when Redis is unavailable")
	})
}

func TestRateLimiterSequential(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	window := 100 * time.Millisecond
	limit := 5
	limiter := NewRateLimiter(client, limit, window)
	key := "sequential-test"
	ctx := context.Background()

	client.FlushAll(ctx)

	var allowed, denied int
	for range limit * 2 {
		result, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		if result.Allowed {
			allowed++
		} else {
			denied++
		}
	}

	assert.Equal(t, limit, allowed, "Should allow exactly %d requests", limit)
	assert.Equal(t, limit, denied, "Should deny exactly %d requests", limit)
}

func TestRateLimiterAccessors(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	window := 5 * time.Second
	limit := 100
	limiter := NewRateLimiter(client, limit, window)

	assert.Equal(t, limit, limiter.Limit())
	assert.Equal(t, window, limiter.Window())
}
