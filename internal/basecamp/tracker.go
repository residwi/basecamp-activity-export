package basecamp

import (
	"context"

	"github.com/residwi/basecamp-activity-export/internal/trace"
)

// trackingContext wraps tracking operations for cleaner code.
type trackingContext struct {
	tracker *trace.Tracer
	recType trace.ResourceType
	active  bool
}

func newTrackingContext(ctx context.Context) *trackingContext {
	tracker, hasTracker := trace.TrackerFromContext(ctx)
	recType, hasRecType := trace.ResourceTypeFromContext(ctx)
	active := hasTracker && hasRecType && tracker != nil
	return &trackingContext{
		tracker: tracker,
		recType: recType,
		active:  active,
	}
}

func (tc *trackingContext) startSpan(span trace.Span) {
	if tc.active {
		tc.tracker.StartSpan(tc.recType, span)
	}
}

func (tc *trackingContext) completeSpan() {
	if tc.active {
		tc.tracker.CompleteSpan(tc.recType)
	}
}

func (tc *trackingContext) fail(err error) {
	if tc.active {
		tc.tracker.FailTrace(tc.recType, err)
	}
}

func (tc *trackingContext) setLastPageFound(lastPage int) {
	if tc.active {
		tc.tracker.SetLastPageFound(tc.recType, lastPage)
	}
}

func (tc *trackingContext) setTotalItems(totalItems int) {
	if tc.active {
		tc.tracker.SetTotalItems(tc.recType, totalItems)
	}
}
