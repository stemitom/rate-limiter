package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now().UnixNano()
	windowStart := now - int64(rl.window)

	pipe := rl.client.Pipeline()

	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprint(windowStart))

	countCmd := pipe.ZCount(ctx, key, fmt.Sprint(windowStart), fmt.Sprint(now))

	pipe.Expire(ctx, key, rl.window+time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("redis pipeline error: %w", err)
	}

	count, err := countCmd.Result()
	if err != nil {
		return false, fmt.Errorf("count result error: %w", err)
	}

	if count < int64(rl.limit) {
		added, err := rl.client.ZAdd(ctx, key, &redis.Z{
			Score:  float64(now),
			Member: now,
		}).Result()
		if err != nil {
			return false, fmt.Errorf("add request error: %w", err)
		}
		return added > 0, nil
	}

	return false, nil
}
