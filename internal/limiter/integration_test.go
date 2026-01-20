//go:build integration

package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRealRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	return client
}

func TestRateLimiterWithRedis(t *testing.T) {
	client := setupRealRedis(t)
	defer client.Close()

	ctx := context.Background()
	client.FlushAll(ctx)

	limiter := NewRateLimiter(client, 5, 5*time.Second)
	key := "integration-test"

	for i := range 5 {
		result, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "Request %d should be allowed", i+1)
		assert.Equal(t, int64(4-i), result.Remaining)
	}

	result, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, int64(0), result.Remaining)

	time.Sleep(5 * time.Second)
	result, err = limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestRateLimiterAtomicity(t *testing.T) {
	client := setupRealRedis(t)
	defer client.Close()

	ctx := context.Background()
	client.FlushAll(ctx)

	window := 5 * time.Second
	limit := 10
	limiter := NewRateLimiter(client, limit, window)
	key := "atomicity-test"

	concurrency := 100
	var wg sync.WaitGroup
	var allowedCount atomic.Int64

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := limiter.Allow(ctx, key)
			if err == nil && result.Allowed {
				allowedCount.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(limit), allowedCount.Load(),
		"Atomicity violated: expected exactly %d allowed, got %d", limit, allowedCount.Load())
}

func TestRateLimiterConcurrency(t *testing.T) {
	client := setupRealRedis(t)
	defer client.Close()

	ctx := context.Background()
	client.FlushAll(ctx)

	window := 1 * time.Second
	limit := 5
	limiter := NewRateLimiter(client, limit, window)
	key := "concurrent-test"

	var wg sync.WaitGroup
	results := make(chan bool, limit*2)

	for range limit * 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := limiter.Allow(ctx, key)
			require.NoError(t, err)
			results <- result.Allowed
		}()
	}

	wg.Wait()
	close(results)

	var allowed, denied int
	for wasAllowed := range results {
		if wasAllowed {
			allowed++
		} else {
			denied++
		}
	}

	assert.Equal(t, limit, allowed, "Should allow exactly %d requests", limit)
	assert.Equal(t, limit, denied, "Should deny exactly %d requests", limit)
}
