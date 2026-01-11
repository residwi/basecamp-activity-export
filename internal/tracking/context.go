package tracking

import "context"

type contextKey int

const (
	resourceTypeKey contextKey = iota
	trackerKey
)

// WithRecordingType adds a ResourceType to the context
func WithResourceType(ctx context.Context, recType ResourceType) context.Context {
	return context.WithValue(ctx, resourceTypeKey, recType)
}

// ResourceTypeFromContext extracts the ResourceType from the context
func ResourceTypeFromContext(ctx context.Context) (ResourceType, bool) {
	recType, ok := ctx.Value(resourceTypeKey).(ResourceType)
	return recType, ok
}

// WithTracker adds a progress tracker to the context
func WithTracker(ctx context.Context, tracker *Tracker) context.Context {
	return context.WithValue(ctx, trackerKey, tracker)
}

// TrackerFromContext extracts the progress tracker from the context
func TrackerFromContext(ctx context.Context) (*Tracker, bool) {
	tracker, ok := ctx.Value(trackerKey).(*Tracker)
	return tracker, ok
}
