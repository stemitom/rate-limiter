package limiter

import (
	"time"
)

type RateLimiterLib interface {
    Allow(key string) (allowed bool, remaining int, reset time.Time, err error)
    UpdateConfig(window time.Duration, limit int) 
    GetRequestCount(key string) (remaining int, err error)
    Reset(key string) error
    RetryAfter(key string) (time.Duration, error)
}