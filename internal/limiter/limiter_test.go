package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	limiter := NewRateLimiter(client, 10, time.Minute)

	key := "client1"
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		allowed, err := limiter.Allow(ctx, key)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, err := limiter.Allow(ctx, key)
	assert.NoError(t, err)
	assert.False(t, allowed)
}
