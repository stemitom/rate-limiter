package limiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)
		assert.Equal(t, StateClosed, cb.State())
		assert.True(t, cb.Allow())
	})

	t.Run("opens after failure threshold", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

		for range 3 {
			cb.RecordFailure()
		}

		assert.Equal(t, StateOpen, cb.State())
		assert.False(t, cb.Allow())
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 50*time.Millisecond)

		for range 3 {
			cb.RecordFailure()
		}
		assert.Equal(t, StateOpen, cb.State())

		time.Sleep(60 * time.Millisecond)

		assert.Equal(t, StateHalfOpen, cb.State())
		assert.True(t, cb.Allow())
	})

	t.Run("closes after success threshold in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 50*time.Millisecond)

		for range 3 {
			cb.RecordFailure()
		}

		time.Sleep(60 * time.Millisecond)
		cb.Allow()

		cb.RecordSuccess()
		assert.Equal(t, StateHalfOpen, cb.State())

		cb.RecordSuccess()
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("reopens on failure in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 2, 50*time.Millisecond)

		cb.RecordFailure()
		assert.Equal(t, StateOpen, cb.State())

		time.Sleep(60 * time.Millisecond)
		cb.Allow()

		cb.RecordFailure()
		assert.Equal(t, StateOpen, cb.State())
	})

	t.Run("success resets failure count", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordSuccess()

		assert.Equal(t, StateClosed, cb.State())

		cb.RecordFailure()
		cb.RecordFailure()
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("reset restores initial state", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 2, 100*time.Millisecond)

		cb.RecordFailure()
		cb.RecordFailure()
		assert.Equal(t, StateOpen, cb.State())

		cb.Reset()
		assert.Equal(t, StateClosed, cb.State())
		assert.True(t, cb.Allow())
	})
}
