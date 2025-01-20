//go:build integration

// does not work (yet)

package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiterWithRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	// Clear test keys
	ctx := context.Background()
	client.FlushAll(ctx)

	limiter := NewRateLimiter(client, 5, 5*time.Second)
	key := "integration-test"

	// Test rate limiting
	for i := 0; i < 5; i++ {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Verify rate limit is enforced
	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Verify key expiration
	time.Sleep(5 * time.Second)
	allowed, err = limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.True(t, allowed)
}
