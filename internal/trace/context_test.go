package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceTypeContext(t *testing.T) {
	t.Run("stores value", func(t *testing.T) {
		ctx := WithResourceType(context.Background(), Messages)
		assert.NotNil(t, ctx)
	})

	t.Run("retrieves stored value", func(t *testing.T) {
		ctx := WithResourceType(context.Background(), Todos)

		rt, ok := ResourceTypeFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, Todos, rt)
	})

	t.Run("returns false when not present", func(t *testing.T) {
		ctx := context.Background()

		rt, ok := ResourceTypeFromContext(ctx)

		assert.False(t, ok)
		assert.Equal(t, ResourceType(""), rt)
	})

	t.Run("returns false for wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), resourceTypeKey, "plain string")

		rt, ok := ResourceTypeFromContext(ctx)

		assert.False(t, ok)
		assert.Equal(t, ResourceType(""), rt)
	})

	t.Run("overwrites previous value", func(t *testing.T) {
		ctx := WithResourceType(context.Background(), Messages)
		ctx = WithResourceType(ctx, Comments)

		rt, ok := ResourceTypeFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, Comments, rt)
	})
}

func TestTrackerContext(t *testing.T) {
	t.Run("stores tracker", func(t *testing.T) {
		tracker := NewTracer()
		ctx := WithTracker(context.Background(), tracker)
		assert.NotNil(t, ctx)
	})

	t.Run("retrieves stored tracker", func(t *testing.T) {
		tracker := NewTracer()
		ctx := WithTracker(context.Background(), tracker)

		retrieved, ok := TrackerFromContext(ctx)

		assert.True(t, ok)
		assert.Same(t, tracker, retrieved)
	})

	t.Run("returns false when not present", func(t *testing.T) {
		ctx := context.Background()

		tracker, ok := TrackerFromContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, tracker)
	})

	t.Run("returns false for wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), trackerKey, "not a tracker")

		tracker, ok := TrackerFromContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, tracker)
	})

	t.Run("returns false for nil value with wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), trackerKey, (*int)(nil))

		tracker, ok := TrackerFromContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, tracker)
	})
}
