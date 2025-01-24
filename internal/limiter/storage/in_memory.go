package storage

import (
	"context"
	"sort"
	"sync"
	"time"
)

type InMemStorage struct {  
    mu    sync.Mutex  
    data  map[string][]time.Time // Key → Sorted timestamps  
}  

// Constructor for in-memory storage
func NewInMemStorage() *InMemStorage {
	return &InMemStorage{
		data: make(map[string][]time.Time),
	}
}

// Atomic CheckAndAdd implementation
func (s *InMemStorage) CheckAndAdd(
	ctx context.Context,
	key string,
	windowStart time.Time,
	timestamp time.Time,
	limit int,
) (bool, error) {
	// Handle context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get existing timestamps (empty slice if key doesn't exist)
	timestamps := s.data[key]

	// Trim old entries
	firstValidIdx := findFirstIndexAfter(timestamps, windowStart)
	trimmed := timestamps[firstValidIdx:]

	// Reject if already at limit
	if len(trimmed) >= limit {
		return false, nil
	}

	// Insert new timestamp (maintain sorted order)
	s.data[key] = insertSorted(trimmed, timestamp)

	return true, nil
}


// GetCount returns the current request count within the window (and trims stale entries).
func (s *InMemStorage) GetCount(
	ctx context.Context,
	key string,
	windowStart time.Time,
) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Trim and update stored data to reflect current window
	timestamps := s.data[key]
	firstValidIdx := findFirstIndexAfter(timestamps, windowStart)
	trimmed := timestamps[firstValidIdx:]
	s.data[key] = trimmed // Persist trimmed state

	return len(trimmed), nil
}


// GetOldestTimestamp returns the oldest request time within the window (or zero time).
func (s *InMemStorage) GetOldestTimestamp(
	ctx context.Context,
	key string,
	windowStart time.Time,
) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Trim and update stored data
	timestamps := s.data[key]
	firstValidIdx := findFirstIndexAfter(timestamps, windowStart)
	trimmed := timestamps[firstValidIdx:]
	s.data[key] = trimmed

	if len(trimmed) == 0 {
		return time.Time{}, nil // No active requests
	}
	return trimmed[0], nil // Oldest timestamp in the window
}

// ResetKey deletes all data for a key.
func (s *InMemStorage) ResetKey(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}


// Helper: Binary search to find first timestamp ≥ windowStart
func findFirstIndexAfter(timestamps []time.Time, cutoff time.Time) int {
	return sort.Search(len(timestamps), func(i int) bool {
		return !timestamps[i].Before(cutoff)
	})
}

// Helper: Insert into sorted slice
func insertSorted(timestamps []time.Time, t time.Time) []time.Time {
	i := sort.Search(len(timestamps), func(i int) bool {
		return !timestamps[i].Before(t)
	})
	return append(timestamps[:i], append([]time.Time{t}, timestamps[i:]...)...)
}

