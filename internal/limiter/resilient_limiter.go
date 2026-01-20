package limiter

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type ResilientRateLimiter struct {
	limiter *RateLimiter
	cb      *CircuitBreaker
}

func NewResilientRateLimiter(
	client *redis.Client,
	limit int,
	window time.Duration,
	failureThreshold int,
	successThreshold int,
	cbTimeout time.Duration,
) *ResilientRateLimiter {
	return &ResilientRateLimiter{
		limiter: NewRateLimiter(client, limit, window),
		cb:      NewCircuitBreaker(failureThreshold, successThreshold, cbTimeout),
	}
}

func (rl *ResilientRateLimiter) Allow(ctx context.Context, key string) (Result, error) {
	if !rl.cb.Allow() {
		return Result{
			Allowed:   false,
			Remaining: 0,
			ResetAt:   time.Now().Add(rl.limiter.Window()),
			Limit:     rl.limiter.Limit(),
		}, ErrCircuitOpen
	}

	result, err := rl.limiter.Allow(ctx, key)
	if err != nil {
		rl.cb.RecordFailure()
		return result, err
	}

	rl.cb.RecordSuccess()
	return result, nil
}

func (rl *ResilientRateLimiter) Limit() int {
	return rl.limiter.Limit()
}

func (rl *ResilientRateLimiter) Window() time.Duration {
	return rl.limiter.Window()
}

func (rl *ResilientRateLimiter) CircuitState() CircuitState {
	return rl.cb.State()
}
