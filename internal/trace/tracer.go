package trace

import (
	"sync"
	"time"
)

// Tracer manages the state of all trace operations
// All methods are safe for concurrent use
type Tracer struct {
	mu        sync.RWMutex
	traces    map[ResourceType]*State
	startTime time.Time
}

func NewTracer() *Tracer {
	return &Tracer{
		startTime: time.Now(),
		traces:    make(map[ResourceType]*State),
	}
}

// RegisterTrace initializes a new trace for the given resource type.
func (t *Tracer) RegisterTrace(rt ResourceType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.traces[rt] = &State{
		Type:      rt,
		Status:    StatusPending,
		StartTime: time.Now(),
		Network: NetworkState{
			Requests:       make(map[string]*RequestInfo),
			RateLimitWaits: make([]RateLimitWait, 0, 4),
		},
	}
}

// StartSpan marks the beginning of a new span within a trace.
func (t *Tracer) StartSpan(rt ResourceType, span Span) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Status = StatusRunning
		state.CurrentSpan = span
		state.SpanStartTime = time.Now()
	}
}

// CompleteSpan marks the current span as completed.
func (t *Tracer) CompleteSpan(rt ResourceType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.SpanEndTime = time.Now()
	}
}

// CompleteTrace marks the trace as completed.
func (t *Tracer) CompleteTrace(rt ResourceType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Status = StatusCompleted
		state.CompletionTime = time.Now()
	}
}

// FailTrace marks the trace as failed with the provided error.
func (t *Tracer) FailTrace(rt ResourceType, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Status = StatusError
		state.Error = err
		state.CompletionTime = time.Now()
	}
}

// SetLastPageFound updates the pagination state.
func (t *Tracer) SetLastPageFound(rt ResourceType, lastPage int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Pagination.LastPageFound = lastPage
	}
}

// SetTotalItems updates the total items count before filtering.
func (t *Tracer) SetTotalItems(rt ResourceType, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Filter.TotalItems = total
	}
}

// SetTotalItemsAfterFilter updates the count after filtering.
func (t *Tracer) SetTotalItemsAfterFilter(rt ResourceType, count int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Filter.TotalItemsAfterFilter = count
	}
}

// StartRequest records the start of a network request.
func (t *Tracer) StartRequest(rt ResourceType, url string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Network.Requests[url] = &RequestInfo{
			URL:       url,
			StartTime: time.Now(),
		}
	}
}

// CompleteRequest records the completion of a network request.
func (t *Tracer) CompleteRequest(rt ResourceType, url string, statusCode int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		if req, ok := state.Network.Requests[url]; ok {
			req.StatusCode = statusCode
			req.Cached = statusCode == 304
			req.EndTime = time.Now()
		}
	}
}

// RecordRateLimitWait records a rate limit wait event.
func (t *Tracer) RecordRateLimitWait(rt ResourceType, duration time.Duration, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.traces[rt]; exists {
		state.Network.RateLimitWaits = append(state.Network.RateLimitWaits, RateLimitWait{
			WaitDuration: duration,
			Reason:       reason,
		})
	}
}

// Snapshot returns an immutable point-in-time copy of the tracer state.
func (t *Tracer) Snapshot() *Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snapshot := &Snapshot{
		startTime: t.startTime,
		traces:    make(map[ResourceType]*State, len(t.traces)),
	}

	for rt, state := range t.traces {
		snapshot.traces[rt] = state.Clone()
	}

	return snapshot
}

// GetElapsedTime returns the duration since the tracer was created.
func (t *Tracer) GetElapsedTime() time.Duration {
	return time.Since(t.startTime)
}
