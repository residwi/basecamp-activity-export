package tracking

import (
	"sync"
	"time"
)

// Tracker manages the state of all trace operations.
type Tracker struct {
	mu        sync.RWMutex
	traces    map[ResourceType]*TraceState
	startTime time.Time
	enabled   bool
}

func NewTracker(enabled bool) *Tracker {
	return &Tracker{
		startTime: time.Now(),
		traces:    make(map[ResourceType]*TraceState),
		enabled:   enabled,
	}
}

func (t *Tracker) IsEnabled() bool {
	return t.enabled
}

func (t *Tracker) GetTrace(recType ResourceType) (*TraceState, bool) {
	if !t.enabled {
		return nil, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	trace, exists := t.traces[recType]
	return trace, exists
}

// RegisterTrace initializes a new trace for the given resource type
func (t *Tracker) RegisterTrace(key ResourceType) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.traces[key] = &TraceState{
		Trace: Trace{
			Type:      key,
			Status:    StatusPending,
			StartTime: time.Now(),
		},
		Network: &NetworkStats{
			Requests:       make(map[string]*RequestInfo),
			RateLimitWaits: make([]*RateLimitWaitInfo, 0),
		},
		Pagination: &PaginationStats{},
		Filter:     &FilterStats{},
	}
}

// CompleteTrace marks the trace as completed
func (t *Tracker) CompleteTrace(recType ResourceType) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.Status = StatusCompleted
		trace.CompletionTime = time.Now()
	}
}

// StartSpan marks the beginning of a new span within a trace
func (t *Tracker) StartSpan(recType ResourceType, span Span) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.Status = StatusRunning
		trace.CurrentSpan = span
		trace.SpanStartTime = time.Now()
	}
}

// CompleteSpan marks the current span as completed
func (t *Tracker) CompleteSpan(recType ResourceType, span Span) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.SpanEndTime = time.Now()
	}
}

// FailTrace marks the trace as failed with the provided error
func (t *Tracker) FailTrace(recType ResourceType, err error) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.Status = StatusError
		trace.Error = err
	}
}

func (rt *TraceState) ensurePagination() {
	if rt.Pagination == nil {
		rt.Pagination = &PaginationStats{}
	}
}

// SetLastPageFound sets the last page found in pagination stats
func (t *Tracker) SetLastPageFound(recType ResourceType, lastPage int) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.ensurePagination()
		trace.Pagination.LastPageFound = lastPage
	}
}

func (rt *TraceState) ensureFilter() {
	if rt.Filter == nil {
		rt.Filter = &FilterStats{}
	}
}

// SetTotalItems sets the total items before filtering in filter stats
func (t *Tracker) SetTotalItems(recType ResourceType, total int) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.ensureFilter()
		trace.Filter.TotalItems = total
	}
}

// SetTotalItemsAfterFilter sets the total items after filtering in filter stats
func (t *Tracker) SetTotalItemsAfterFilter(recType ResourceType, afterFilter int) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.ensureFilter()
		trace.Filter.TotalItemsAfterFilter = afterFilter
	}
}

func (rt *TraceState) ensureNetwork() {
	if rt.Network == nil {
		rt.Network = &NetworkStats{
			Requests:       make(map[string]*RequestInfo),
			RateLimitWaits: make([]*RateLimitWaitInfo, 0),
		}
	}
}

// StartRequest records the start of a network request
func (t *Tracker) StartRequest(recType ResourceType, url string) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.ensureNetwork()
		trace.Network.Requests[url] = &RequestInfo{
			URL:       url,
			StartTime: time.Now(),
		}
	}
}

// CompleteRequest records the completion of a network request
func (t *Tracker) CompleteRequest(recType ResourceType, url string, statusCode int) {
	if !t.enabled {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.ensureNetwork()

		if req, exists := trace.Network.Requests[url]; exists {
			if statusCode == 304 {
				req.Cached = true
			}

			req.StatusCode = statusCode
			req.EndTime = time.Now()
		}
	}
}

func (t *Tracker) RecordRateLimitWait(recType ResourceType, duration time.Duration, reason string) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if trace, exists := t.traces[recType]; exists {
		trace.ensureNetwork()
		trace.Network.RateLimitWaits = append(trace.Network.RateLimitWaits, &RateLimitWaitInfo{
			WaitDuration: duration,
			Reason:       reason,
		})
	}
}

// GetState returns a deep copy of the current tracker state
func (t *Tracker) GetState() *Tracker {
	if !t.enabled {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	state := &Tracker{
		startTime: t.startTime,
		traces:    make(map[ResourceType]*TraceState, len(t.traces)),
		enabled:   t.enabled,
	}

	for k, v := range t.traces {
		traceCopy := *v
		if v.Network != nil {
			traceCopy.Network = &NetworkStats{}

			// Deep copy Requests
			if v.Network.Requests != nil {
				traceCopy.Network.Requests = make(map[string]*RequestInfo, len(v.Network.Requests))
				for url, req := range v.Network.Requests {
					reqCopy := *req
					traceCopy.Network.Requests[url] = &reqCopy
				}
			}

			// Deep copy RateLimitWaits
			if v.Network.RateLimitWaits != nil {
				traceCopy.Network.RateLimitWaits = make([]*RateLimitWaitInfo, 0, len(v.Network.RateLimitWaits))
				for _, wait := range v.Network.RateLimitWaits {
					waitCopy := *wait
					traceCopy.Network.RateLimitWaits = append(traceCopy.Network.RateLimitWaits, &waitCopy)
				}
			}
		}

		if v.Pagination != nil {
			paginationCopy := *v.Pagination
			traceCopy.Pagination = &paginationCopy
		}

		if v.Filter != nil {
			filterCopy := *v.Filter
			traceCopy.Filter = &filterCopy
		}

		state.traces[k] = &traceCopy
	}

	return state
}

// GetElapsedTime returns the elapsed time since the tracker was created
func (t *Tracker) GetElapsedTime() time.Duration {
	if !t.enabled {
		return 0
	}
	return time.Since(t.startTime)
}
