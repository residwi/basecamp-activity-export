package tracking

import "time"

type ResourceType int

const (
	Messages ResourceType = iota
	Todos
	Todolists
	Cards
	Comments
	QuestionAnswers
)

var RecordingTypes = []ResourceType{
	Messages,
	Todos,
	Todolists,
	Cards,
	Comments,
	QuestionAnswers,
}

// Span represents sequential stages within a trace's lifecycle
type Span int

const (
	SpanFindLastPage Span = iota
	SpanFetchData
	SpanFilter
)

type OperationStatus int

const (
	StatusPending OperationStatus = iota
	StatusRunning
	StatusCompleted
	StatusError
)

type RequestInfo struct {
	URL        string
	StatusCode int
	Cached     bool
	StartTime  time.Time
	EndTime    time.Time
}

type RateLimitWaitInfo struct {
	WaitDuration time.Duration
	Reason       string // e.g., "429 Too Many Requests" or "Proactive rate limit"
}

type Trace struct {
	Type        ResourceType
	Status      OperationStatus
	CurrentSpan Span
	Error       error

	SpanStartTime time.Time
	SpanEndTime   time.Time

	StartTime      time.Time
	CompletionTime time.Time
}

type NetworkStats struct {
	Requests       map[string]*RequestInfo
	RateLimitWaits []*RateLimitWaitInfo
}

type PaginationStats struct {
	LastPageFound int
}

type FilterStats struct {
	TotalItems            int
	TotalItemsAfterFilter int
}

type TraceState struct {
	Trace

	Network    *NetworkStats
	Pagination *PaginationStats
	Filter     *FilterStats
}

func (ns *NetworkStats) TotalRequests() int {
	totalRequest := 0

	for _, req := range ns.Requests {
		if req.StatusCode != 0 {
			totalRequest++
		}
	}

	return totalRequest
}
