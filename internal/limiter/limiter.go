package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Lua script for atomic sliding window rate limiting
// This ensures the check-and-increment operation is atomic, preventing race conditions
const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local windowStart = now - window

-- Remove expired entries (outside the sliding window)
redis.call('ZREMRANGEBYSCORE', key, '0', windowStart)

-- Count current entries in the window
local count = redis.call('ZCOUNT', key, windowStart, now)

-- Calculate reset time (when the oldest entry in current window expires)
local resetAt = now + window

if count < limit then
    -- Add new entry with current timestamp as both score and member
    redis.call('ZADD', key, now, now)
    -- Set expiration to window duration + buffer to ensure cleanup
    redis.call('EXPIRE', key, math.ceil(window / 1000000000) + 60)
    -- Return: allowed=1, remaining requests, reset timestamp
    return {1, limit - count - 1, resetAt}
end

-- Rate limited: return allowed=0, remaining=0, reset timestamp
return {0, 0, resetAt}
`

// Result represents the outcome of a rate limit check
type Result struct {
	// Allowed indicates whether the request should be permitted
	Allowed bool
	// Remaining is the number of requests left in the current window
	Remaining int64
	// ResetAt is when the current window resets
	ResetAt time.Time
	// Limit is the maximum number of requests per window
	Limit int
}

// RateLimiter implements a distributed sliding window rate limiter using Redis
type RateLimiter struct {
	client *redis.Client
	script *redis.Script
	limit  int
	window time.Duration
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		script: redis.NewScript(slidingWindowScript),
		limit:  limit,
		window: window,
	}
}

// Limit returns the configured request limit per window
func (rl *RateLimiter) Limit() int {
	return rl.limit
}

// Window returns the configured time window duration
func (rl *RateLimiter) Window() time.Duration {
	return rl.window
}

// Allow checks if a request identified by key should be allowed
// It atomically checks the current count and adds a new entry if under the limit
func (rl *RateLimiter) Allow(ctx context.Context, key string) (Result, error) {
	now := time.Now().UnixNano()

	res, err := rl.script.Run(ctx, rl.client, []string{key},
		now,
		int64(rl.window),
		rl.limit,
	).Slice()

	if err != nil {
		return Result{
			Allowed:   false,
			Remaining: 0,
			ResetAt:   time.Now().Add(rl.window),
			Limit:     rl.limit,
		}, fmt.Errorf("rate limiter script error: %w", err)
	}

	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	resetAt := time.Unix(0, res[2].(int64))

	return Result{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   resetAt,
		Limit:     rl.limit,
	}, nil
}
