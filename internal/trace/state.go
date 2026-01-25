package trace

import "time"

// RequestInfo contains details about a single HTTP request
type RequestInfo struct {
	URL        string
	StatusCode int
	Cached     bool
	StartTime  time.Time
	EndTime    time.Time
}

// RateLimitWait records a rate limit wait event
type RateLimitWait struct {
	WaitDuration time.Duration
	Reason       string // e.g., "429 Too Many Requests" or "Proactive rate limit"
}

// NetworkState holds network-related statistics for a trace
type NetworkState struct {
	Requests       map[string]*RequestInfo
	RateLimitWaits []RateLimitWait
}

// CompletedRequests returns the count of requests that have completed
func (ns *NetworkState) CompletedRequests() int {
	count := 0
	for _, req := range ns.Requests {
		if req.StatusCode != 0 {
			count++
		}
	}
	return count
}

// PaginationState holds pagination-related statistics
type PaginationState struct {
	LastPageFound int
}

// FilterState holds filter-related statistics
type FilterState struct {
	TotalItems            int
	TotalItemsAfterFilter int
}

// State represents the complete state of a single resource trace
type State struct {
	Type        ResourceType
	Status      Status
	CurrentSpan Span
	Error       error

	StartTime      time.Time
	CompletionTime time.Time
	SpanStartTime  time.Time
	SpanEndTime    time.Time

	Network    NetworkState
	Pagination PaginationState
	Filter     FilterState
}

// Clone creates a deep copy of the State
func (ts *State) Clone() *State {
	clone := &State{
		Type:           ts.Type,
		Status:         ts.Status,
		CurrentSpan:    ts.CurrentSpan,
		Error:          ts.Error,
		StartTime:      ts.StartTime,
		CompletionTime: ts.CompletionTime,
		SpanStartTime:  ts.SpanStartTime,
		SpanEndTime:    ts.SpanEndTime,
		Pagination:     ts.Pagination,
		Filter:         ts.Filter,
		Network: NetworkState{
			Requests:       make(map[string]*RequestInfo, len(ts.Network.Requests)),
			RateLimitWaits: make([]RateLimitWait, len(ts.Network.RateLimitWaits)),
		},
	}

	for url, req := range ts.Network.Requests {
		reqCopy := *req
		clone.Network.Requests[url] = &reqCopy
	}

	copy(clone.Network.RateLimitWaits, ts.Network.RateLimitWaits)

	return clone
}

// Snapshot represents an immutable point-in-time view of the tracer state
type Snapshot struct {
	traces    map[ResourceType]*State
	startTime time.Time
}

// GetTrace returns the trace state for the given resource type
func (s *Snapshot) GetTrace(rt ResourceType) (*State, bool) {
	state, exists := s.traces[rt]
	return state, exists
}

// GetElapsedTime returns the duration since tracking started
func (s *Snapshot) GetElapsedTime() time.Duration {
	return time.Since(s.startTime)
}

// Traces returns all trace states in the snapshot
func (s *Snapshot) Traces() map[ResourceType]*State {
	return s.traces
}
