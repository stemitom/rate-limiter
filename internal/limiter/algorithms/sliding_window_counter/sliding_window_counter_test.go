package slidingwindowcounter

import (
	"testing"
	"time"

	"github.com/stemitom/rate-limiter/internal/limiter/storage"
)

func TestSlidingWindowCounter(t *testing.T) {
	tests := []struct {
		name           string
		window         time.Duration
		limit          int
		requestDelays  []time.Duration // delays between requests
		expectedAllow  []bool          // expected results for each request
	}{
		{
			name:           "Basic rate limiting",
			window:         time.Second,
			limit:          3,
			requestDelays:  []time.Duration{0, 0, 0, 0},  // 4 immediate requests
			expectedAllow:  []bool{true, true, true, false}, // first 3 allowed, 4th blocked
		},
		{
			name:           "Window sliding",
			window:         time.Second,
			limit:          2,
			requestDelays:  []time.Duration{0, 0, time.Second, 0},  // 2 requests, wait 1s, 2 more
			expectedAllow:  []bool{true, true, true, true},        // all allowed as window slides
		},
		{
			name:           "Zero limit",
			window:         time.Second,
			limit:          0,
			requestDelays:  []time.Duration{0, 0},
			expectedAllow:  []bool{false, false}, // all requests should be blocked
		},
		{
			name:           "Very small window",
			window:         time.Microsecond,
			limit:          1,
			requestDelays:  []time.Duration{0, time.Millisecond},
			expectedAllow:  []bool{true, true}, // both allowed as window is tiny
		},
		{
			name:           "Window boundary",
			window:         100 * time.Millisecond,
			limit:          1,
			requestDelays:  []time.Duration{0, 99 * time.Millisecond, time.Millisecond}, // just before and after window
			expectedAllow:  []bool{true, false, true}, // first allowed, second blocked, third allowed after window
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storage.NewInMemStorage()
			limiter := NewSlidingWindowCounter(store, tt.window, tt.limit)

			for i, delay := range tt.requestDelays {
				time.Sleep(delay)
				allowed, err := limiter.Allow("test-key")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if allowed != tt.expectedAllow[i] {
					t.Errorf("request %d: got %v, want %v", i+1, allowed, tt.expectedAllow[i])
				}
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	store := storage.NewInMemStorage()
	limiter := NewSlidingWindowCounter(store, time.Second, 1000)

	const numGoroutines = 10
	const requestsPerGoroutine = 100

	done := make(chan bool)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < requestsPerGoroutine; j++ {
				_, err := limiter.Allow("concurrent-test")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify the total count
	count, err := limiter.GetRequestCount("concurrent-test")
	if err != nil {
		t.Fatalf("unexpected error getting count: %v", err)
	}
	if count != numGoroutines*requestsPerGoroutine {
		t.Errorf("got count %d, want %d", count, numGoroutines*requestsPerGoroutine)
	}
}

func TestMultipleKeys(t *testing.T) {
	store := storage.NewInMemStorage()
	limiter := NewSlidingWindowCounter(store, time.Second, 1)

	// Test that different keys are rate limited independently
	keys := []string{"key1", "key2", "key3"}

	// Each key should allow exactly one request
	for _, key := range keys {
		// First request should be allowed
		allowed, err := limiter.Allow(key)
		if err != nil {
			t.Fatalf("unexpected error for key %s: %v", key, err)
		}
		if !allowed {
			t.Errorf("first request for key %s should be allowed", key)
		}

		// Second request should be blocked
		allowed, err = limiter.Allow(key)
		if err != nil {
			t.Fatalf("unexpected error for key %s: %v", key, err)
		}
		if allowed {
			t.Errorf("second request for key %s should be blocked", key)
		}
	}

	// Verify counts for each key
	for _, key := range keys {
		count, err := limiter.GetRequestCount(key)
		if err != nil {
			t.Fatalf("unexpected error getting count for key %s: %v", key, err)
		}
		if count != 1 {
			t.Errorf("expected count 1 for key %s, got %d", key, count)
		}
	}
}

func TestUpdateConfig(t *testing.T) {
	store := storage.NewInMemStorage()
	limiter := NewSlidingWindowCounter(store, time.Second, 2)

	// Initial config should allow 2 requests
	allowed1, _ := limiter.Allow("test-key")
	allowed2, _ := limiter.Allow("test-key")
	allowed3, _ := limiter.Allow("test-key")
	if !allowed1 || !allowed2 || allowed3 {
		t.Error("unexpected results with initial config")
	}

	// Update config to allow 3 requests
	limiter.UpdateConfig(time.Second, 3)

	// Should allow one more request now
	allowed4, _ := limiter.Allow("test-key")
	if !allowed4 {
		t.Error("request should be allowed after increasing limit")
	}

	// Test rapid config updates
	for i := 0; i < 100; i++ {
		go func(val int) {
			limiter.UpdateConfig(time.Duration(val+1)*time.Millisecond, val+1)
		}(i)
	}
	// Allow time for updates to complete
	time.Sleep(10 * time.Millisecond)

	// Verify limiter still works
	allowed, err := limiter.Allow("test-key")
	if err != nil {
		t.Errorf("unexpected error after rapid updates: %v", err)
	}
	// Verify we can still make requests
	if !allowed {
		t.Error("limiter should still allow requests after updates")
	}
}

func TestReset(t *testing.T) {
	store := storage.NewInMemStorage()
	limiter := NewSlidingWindowCounter(store, time.Second, 1)

	// Use up the limit
	allowed1, _ := limiter.Allow("test-key")
	allowed2, _ := limiter.Allow("test-key")
	if !allowed1 || allowed2 {
		t.Error("unexpected results before reset")
	}

	// Reset the counter
	err := limiter.Reset("test-key")
	if err != nil {
		t.Fatalf("unexpected error on reset: %v", err)
	}

	// Should be allowed after reset
	allowed3, _ := limiter.Allow("test-key")
	if !allowed3 {
		t.Error("request should be allowed after reset")
	}
}

func TestRetryAfter(t *testing.T) {
	store := storage.NewInMemStorage()
	window := time.Second
	limiter := NewSlidingWindowCounter(store, window, 1)

	// Use up the limit
	_, err := limiter.Allow("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get retry duration
	retry, err := limiter.RetryAfter("test-key")
	if err != nil {
		t.Fatalf("unexpected error getting retry duration: %v", err)
	}

	if retry > window {
		t.Errorf("retry duration %v should not be greater than window %v", retry, window)
	}
	if retry <= 0 {
		t.Error("retry duration should be positive when rate limited")
	}
}
