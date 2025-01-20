package limiter

import (
	"context"
	"math/rand"
	"sync"
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

		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, key)
			assert.NoError(t, err)
			assert.True(t, allowed, "Request %d should be allowed", i+1)
		}
	})

	t.Run("exceeding limit", func(t *testing.T) {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.False(t, allowed, "Request should be denied when exceeding limit")
	})

	t.Run("sliding window behavior", func(t *testing.T) {
		client.FlushAll(ctx)

		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, key)
			require.NoError(t, err)
			require.True(t, allowed)
		}

		time.Sleep(window / 2)

		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.False(t, allowed, "Should still be limited before window expires")

		time.Sleep(window)

		allowed, err = limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed, "Should allow requests after window expires")
	})

	t.Run("multiple keys", func(t *testing.T) {
		client.FlushAll(ctx)

		key1 := "client1"
		key2 := "client2"

		for i := 0; i < limit; i++ {
			allowed, err := limiter.Allow(ctx, key1)
			require.NoError(t, err)
			require.True(t, allowed)
		}

		allowed, err := limiter.Allow(ctx, key2)
		assert.NoError(t, err)
		assert.True(t, allowed, "Different keys should have separate limits")
	})

	t.Run("error handling", func(t *testing.T) {
		client.Close()

		allowed, err := limiter.Allow(ctx, key)
		assert.Error(t, err, "Should return error when Redis is unavailable")
		assert.False(t, allowed, "Should deny requests when Redis is unavailable")
	})
}

func TestRateLimiterConcurrency(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	window := 100 * time.Millisecond
	limit := 5
	limiter := NewRateLimiter(client, limit, window)
	key := "concurrent-test"
	ctx := context.Background()

	client.FlushAll(ctx)

	var wg sync.WaitGroup
	results := make(chan bool, limit*2)

	// Run requests concurrently
	for i := 0; i < limit*2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Add small random delay to better simulate real-world conditions
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			allowed, err := limiter.Allow(ctx, key)
			require.NoError(t, err)
			results <- allowed
		}()
	}

	wg.Wait()
	close(results)

	var allowed, denied int
	for result := range results {
		if result {
			allowed++
		} else {
			denied++
		}
	}

	assert.Equal(t, limit, allowed, "Should allow exactly %d requests", limit)
	assert.Equal(t, limit, denied, "Should deny exactly %d requests", limit)
}
