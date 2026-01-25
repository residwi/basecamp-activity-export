package trace

import "context"

const (
	resourceTypeKey string = "resourceType"
	trackerKey      string = "tracker"
)

// WithResourceType adds a ResourceType to the context
func WithResourceType(ctx context.Context, rt ResourceType) context.Context {
	return context.WithValue(ctx, resourceTypeKey, rt)
}

// ResourceTypeFromContext extracts the ResourceType from the context
func ResourceTypeFromContext(ctx context.Context) (ResourceType, bool) {
	rt, ok := ctx.Value(resourceTypeKey).(ResourceType)
	return rt, ok
}

// WithTracker adds a Tracer to the context
func WithTracker(ctx context.Context, tracker *Tracer) context.Context {
	return context.WithValue(ctx, trackerKey, tracker)
}

// TrackerFromContext extracts the Tracer from the context
func TrackerFromContext(ctx context.Context) (*Tracer, bool) {
	tracker, ok := ctx.Value(trackerKey).(*Tracer)
	return tracker, ok
}
