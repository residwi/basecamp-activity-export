package trace

// Span represents sequential stages within a trace's lifecycle.
type Span string

const (
	SpanFindLastPage Span = "find_last_page"
	SpanFetchData    Span = "fetch_data"
	SpanFilter       Span = "filter"
)

// Status represents the current state of an operation.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusCompleted
	StatusError
)
