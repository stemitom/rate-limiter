package slidingwindowcounter

import (
	"context"
	"sync"
	"time"

	"github.com/stemitom/rate-limiter/internal/limiter/storage"
)

type SlidingWindowCounter struct {
    mu      sync.RWMutex         // For thread-safe config updates
    storage  storage.Storage
    window   time.Duration  
    limit    int  
}  

func NewSlidingWindowCounter(  
    storage storage.Storage,  
    window time.Duration,  
    limit int,  
) *SlidingWindowCounter {  
    return &SlidingWindowCounter{  
        storage: storage,  
        window:  window,  
        limit:   limit,  
    }  
}  

func (s *SlidingWindowCounter) Allow(key string) (bool, error) {
    s.mu.RLock()
	defer s.mu.RUnlock()

    now := time.Now()  
    windowStart := now.Add(-s.window)  

    return s.storage.CheckAndAdd(  
        context.Background(),  
        key,  
        windowStart,  
        now,  
        s.limit,  
    )  
}

// RetryAfter returns the duration until the next allowed request.
func (s *SlidingWindowCounter) RetryAfter(key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	windowStart := now.Add(-s.window)

	oldest, err := s.storage.GetOldestTimestamp(
		context.Background(),
		key,
		windowStart,
	)
	if err != nil {
		return 0, err
	}

	if oldest.IsZero() {
		return 0, nil // No active requests in the window
	}
	retryTime := oldest.Add(s.window)
	return retryTime.Sub(now), nil
}


// GetRequestCount returns the current number of requests in the window.
func (s *SlidingWindowCounter) GetRequestCount(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	windowStart := time.Now().Add(-s.window)
	return s.storage.GetCount(context.Background(), key, windowStart)
}

// UpdateConfig safely updates the window size and request limit.
func (s *SlidingWindowCounter) UpdateConfig(window time.Duration, limit int) {
    s.mu.Lock()
	defer s.mu.Unlock()
	s.window = window
	s.limit = limit
}

func (s *SlidingWindowCounter) Reset(key string) error {
	return s.storage.ResetKey(context.Background(), key)
}


