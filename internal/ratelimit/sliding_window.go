package ratelimit

import (
	"sync"
	"time"
)

type Limiter interface {
	Allow() bool
}

// SlidingWindowLimiter implements rate limiting using a sliding window counter algorithm.
// This provides accurate rate limiting with O(1) memory and time complexity.
//
// The algorithm maintains two windows (previous and current) and uses weighted counting
// to provide smooth rate limiting across window boundaries:
//
//	weightedCount = prevCount * (1 - elapsed/window) + currCount
//
// This prevents the "burst at boundary" problem of fixed windows while maintaining
// constant memory usage unlike sliding log approaches.
type SlidingWindowLimiter struct {
	mu sync.Mutex

	limit  int           // Maximum requests allowed per window
	window time.Duration // Time window duration

	currWindow time.Time // Start time of current window
	currCount  int       // Request count in current window
	prevCount  int       // Request count in previous window
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter.
//
// Parameters:
//   - limit: Maximum number of requests allowed per window
//   - window: Duration of the time window
func NewSlidingWindowLimiter(limit int, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		limit:      limit,
		window:     window,
		currWindow: time.Now().Truncate(window),
		currCount:  0,
		prevCount:  0,
	}
}

// Allow checks if a request can proceed under the rate limit.
//
// Returns true if the request is allowed, false if rate limited.
// This method is thread-safe and can be called concurrently.
//
// The sliding window algorithm ensures smooth rate limiting by considering
// both the current and previous time windows with appropriate weighting.
func (l *SlidingWindowLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	currWindowStart := now.Truncate(l.window)

	// Slide window forward if we've entered a new window
	if currWindowStart.After(l.currWindow) {
		// If we've skipped more than one window, reset previous count
		if currWindowStart.Sub(l.currWindow) >= l.window*2 {
			l.prevCount = 0
		} else {
			l.prevCount = l.currCount
		}
		l.currCount = 0
		l.currWindow = currWindowStart
	}

	// Calculate weighted request count across sliding window
	elapsed := now.Sub(l.currWindow)
	prevWeight := 1 - float64(elapsed)/float64(l.window)
	weightedCount := float64(l.prevCount)*prevWeight + float64(l.currCount)

	if weightedCount < float64(l.limit) {
		l.currCount++
		return true
	}

	return false
}
