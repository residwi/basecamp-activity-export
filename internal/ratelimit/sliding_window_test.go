package ratelimit

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlidingWindowLimiter(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{
			name:   "standard limits",
			limit:  50,
			window: 10 * time.Second,
		},
		{
			name:   "strict limits",
			limit:  10,
			window: 1 * time.Second,
		},
		{
			name:   "relaxed limits",
			limit:  1000,
			window: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewSlidingWindowLimiter(tt.limit, tt.window)

			require.NotNil(t, limiter, "Expected non-nil limiter")
			assert.Equal(t, tt.limit, limiter.limit, "Limit should match")
			assert.Equal(t, tt.window, limiter.window, "Window should match")
			assert.Equal(t, 0, limiter.currCount, "Initial currCount should be 0")
			assert.Equal(t, 0, limiter.prevCount, "Initial prevCount should be 0")
		})
	}
}

func TestAllow(t *testing.T) {
	t.Run("exceeds limit", func(t *testing.T) {
		limiter := NewSlidingWindowLimiter(5, 1*time.Second)

		for i := range 5 {
			assert.True(t, limiter.Allow(), "Request %d should be allowed", i+1)
		}

		for i := 5; i < 10; i++ {
			assert.False(t, limiter.Allow(), "Request %d should be denied (exceeds limit)", i+1)
		}
	})

	t.Run("window sliding", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limit := 50
			limiter := NewSlidingWindowLimiter(limit, 10*time.Second)

			for i := range limit {
				assert.True(t, limiter.Allow(), "Request %d should be allowed in first window", i+1)
			}
			assert.False(t, limiter.Allow(), "Should be rate limited in first window")

			// enter new window
			time.Sleep(10 * time.Second)

			// Spread requests evenly over the new window
			delay := 10 * time.Second / time.Duration(limit)
			for i := range limit {
				time.Sleep(delay)
				assert.True(t, limiter.Allow(), "Request %d should be allowed in new window", i+1)
			}
		})
	})

	t.Run("sliding boundary", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewSlidingWindowLimiter(10, 20*time.Millisecond)

			for i := range 5 {
				assert.True(t, limiter.Allow(), "Request %d should be allowed", i+1)
			}

			time.Sleep(10 * time.Millisecond)

			// Weighted count = prevCount * 0.5 + currCount
			// = 10 * 0.5 + 0 = 5
			// So we should be able to make ~5 more requests
			allowed := 0
			for range 10 {
				if limiter.Allow() {
					allowed++
				}
			}

			assert.Equal(t, 5, allowed, "Expected 5 requests allowed at sliding boundary")
		})
	})

	t.Run("concurrent requests", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewSlidingWindowLimiter(10, 1*time.Second)

			results := make(chan bool, 20)

			for range 20 {
				go func() {
					results <- limiter.Allow()
				}()
			}

			allowed := 0
			for range 20 {
				if <-results {
					allowed++
				}
			}

			synctest.Wait()

			assert.Equal(t, 10, allowed, "Expected 10 allowed requests")

			time.Sleep(1 * time.Second)

			for i := range 10 {
				time.Sleep(100 * time.Millisecond)
				assert.True(t, limiter.Allow(), "Request %d should be allowed after window", i+1)
			}
		})
	})

	t.Run("window transition", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewSlidingWindowLimiter(5, 11*time.Millisecond)

			for i := range 3 {
				assert.True(t, limiter.Allow(), "Request %d should be allowed in first window", i+1)
			}

			time.Sleep(12 * time.Millisecond)

			for i := range 5 {
				time.Sleep(1 * time.Millisecond)
				assert.True(t, limiter.Allow(), "Request %d should be allowed in new window", i+1)
			}

			assert.False(t, limiter.Allow(), "Request 6 should be denied in new window")
		})
	})

	t.Run("multiple windows", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limiter := NewSlidingWindowLimiter(5, 50*time.Millisecond)

			for window := range 3 {
				for i := range 5 {
					assert.True(t, limiter.Allow(), "Window %d, request %d should be allowed", window, i+1)
					// Spread requests within the window
					time.Sleep(10 * time.Millisecond)
				}

				assert.False(t, limiter.Allow(), "Window %d should be rate limited", window)

				// Wait for next window
				time.Sleep(50 * time.Millisecond)
			}
		})
	})

	t.Run("long idle time", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			limit := 5
			windowSize := 100 * time.Millisecond

			limiter := NewSlidingWindowLimiter(limit, windowSize)
			for i := range limit {
				assert.True(t, limiter.Allow(), "Request %d should be allowed", i+1)
			}

			// Sleeping for 5 windows (simulating long idle time)
			time.Sleep(500 * time.Millisecond)

			// We sleep a tiny bit more to ensure we are partly into the current window
			time.Sleep(10 * time.Millisecond)

			for i := range limit {
				assert.True(t, limiter.Allow(), "Request %d after idle should be allowed", i+1)
			}
		})
	})
}

func BenchmarkSlidingWindowLimiter_Allow(b *testing.B) {
	limiter := NewSlidingWindowLimiter(1000000, 1*time.Second)

	for b.Loop() {
		limiter.Allow()
	}
}

func BenchmarkSlidingWindowLimiter_Allow_Concurrent(b *testing.B) {
	limiter := NewSlidingWindowLimiter(1000000, 1*time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}
