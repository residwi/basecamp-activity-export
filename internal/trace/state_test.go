package trace

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkState_CompletedRequests(t *testing.T) {
	tests := []struct {
		name     string
		requests map[string]*RequestInfo
		expected int
	}{
		{
			name:     "empty requests map",
			requests: map[string]*RequestInfo{},
			expected: 0,
		},
		{
			name: "no completed requests (all status code 0)",
			requests: map[string]*RequestInfo{
				"url1": {StatusCode: 0},
				"url2": {StatusCode: 0},
			},
			expected: 0,
		},
		{
			name: "all requests completed",
			requests: map[string]*RequestInfo{
				"url1": {StatusCode: 200},
				"url2": {StatusCode: 304},
				"url3": {StatusCode: 404},
			},
			expected: 3,
		},
		{
			name: "mixed completed and pending",
			requests: map[string]*RequestInfo{
				"url1": {StatusCode: 200},
				"url2": {StatusCode: 0},
				"url3": {StatusCode: 500},
				"url4": {StatusCode: 0},
			},
			expected: 2,
		},
		{
			name: "single completed request",
			requests: map[string]*RequestInfo{
				"url1": {StatusCode: 201},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &NetworkState{Requests: tt.requests}

			result := ns.CompletedRequests()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestState_Clone(t *testing.T) {
	t.Run("clones all scalar fields correctly", func(t *testing.T) {
		now := time.Now()
		testErr := errors.New("test error")
		original := &State{
			Type:           Messages,
			Status:         StatusRunning,
			CurrentSpan:    SpanFetchData,
			Error:          testErr,
			StartTime:      now,
			CompletionTime: now.Add(time.Minute),
			SpanStartTime:  now.Add(10 * time.Second),
			SpanEndTime:    now.Add(30 * time.Second),
			Pagination:     PaginationState{LastPageFound: 15},
			Filter:         FilterState{TotalItems: 100, TotalItemsAfterFilter: 75},
			Network: NetworkState{
				Requests:       make(map[string]*RequestInfo),
				RateLimitWaits: make([]RateLimitWait, 0),
			},
		}

		clone := original.Clone()

		assert.NotSame(t, original, clone)
		assert.Equal(t, original.Type, clone.Type)
		assert.Equal(t, original.Status, clone.Status)
		assert.Equal(t, original.CurrentSpan, clone.CurrentSpan)
		assert.Equal(t, original.Error, clone.Error)
		assert.Equal(t, original.StartTime, clone.StartTime)
		assert.Equal(t, original.CompletionTime, clone.CompletionTime)
		assert.Equal(t, original.SpanStartTime, clone.SpanStartTime)
		assert.Equal(t, original.SpanEndTime, clone.SpanEndTime)
		assert.Equal(t, original.Pagination, clone.Pagination)
		assert.Equal(t, original.Filter, clone.Filter)
	})

	t.Run("creates deep copy of Network.Requests", func(t *testing.T) {
		original := &State{
			Network: NetworkState{
				Requests: map[string]*RequestInfo{
					"url1": {URL: "url1", StatusCode: 200, Cached: true},
					"url2": {URL: "url2", StatusCode: 304, Cached: false},
				},
				RateLimitWaits: make([]RateLimitWait, 0),
			},
		}

		clone := original.Clone()

		assert.Len(t, clone.Network.Requests, 2)

		// each requestinfo should be a different pointer but equal value
		for url, origReq := range original.Network.Requests {
			cloneReq := clone.Network.Requests[url]
			assert.NotSame(t, origReq, cloneReq)
			assert.Equal(t, *origReq, *cloneReq)
		}

		// verify maps are independent by modifying clone
		clone.Network.Requests["url1"].StatusCode = 999
		clone.Network.Requests["url3"] = &RequestInfo{URL: "url3"}
		assert.Equal(t, 200, original.Network.Requests["url1"].StatusCode)
		assert.NotContains(t, original.Network.Requests, "url3")
	})

	t.Run("creates deep copy of Network.RateLimitWaits", func(t *testing.T) {
		original := &State{
			Network: NetworkState{
				Requests: make(map[string]*RequestInfo),
				RateLimitWaits: []RateLimitWait{
					{WaitDuration: time.Second, Reason: "429"},
					{WaitDuration: 2 * time.Second, Reason: "proactive"},
				},
			},
		}

		clone := original.Clone()

		assert.Equal(t, original.Network.RateLimitWaits, clone.Network.RateLimitWaits)
		assert.Len(t, clone.Network.RateLimitWaits, 2)

		// modifying clone's slice shouldn't affect original
		clone.Network.RateLimitWaits[0].Reason = "modified"
		assert.Equal(t, "429", original.Network.RateLimitWaits[0].Reason)
	})

	t.Run("modifications to clone do not affect original", func(t *testing.T) {
		original := &State{
			Type:   Messages,
			Status: StatusPending,
			Network: NetworkState{
				Requests: map[string]*RequestInfo{
					"url1": {URL: "url1", StatusCode: 200},
				},
				RateLimitWaits: []RateLimitWait{{WaitDuration: time.Second}},
			},
			Pagination: PaginationState{LastPageFound: 10},
			Filter:     FilterState{TotalItems: 50},
		}

		clone := original.Clone()

		clone.Type = Todos
		clone.Status = StatusCompleted
		clone.Network.Requests["url1"].StatusCode = 404
		clone.Network.Requests["url2"] = &RequestInfo{URL: "url2"}
		clone.Network.RateLimitWaits = append(clone.Network.RateLimitWaits, RateLimitWait{})
		clone.Pagination.LastPageFound = 20
		clone.Filter.TotalItems = 100

		assert.Equal(t, Messages, original.Type)
		assert.Equal(t, StatusPending, original.Status)
		assert.Equal(t, 200, original.Network.Requests["url1"].StatusCode)
		assert.Len(t, original.Network.Requests, 1)
		assert.Len(t, original.Network.RateLimitWaits, 1)
		assert.Equal(t, 10, original.Pagination.LastPageFound)
		assert.Equal(t, 50, original.Filter.TotalItems)
	})

	t.Run("handles empty collections", func(t *testing.T) {
		original := &State{
			Network: NetworkState{
				Requests:       map[string]*RequestInfo{},
				RateLimitWaits: []RateLimitWait{},
			},
		}

		clone := original.Clone()

		assert.NotNil(t, clone.Network.Requests)
		assert.NotNil(t, clone.Network.RateLimitWaits)
		assert.Empty(t, clone.Network.Requests)
		assert.Empty(t, clone.Network.RateLimitWaits)
	})

	t.Run("handles nil error", func(t *testing.T) {
		original := &State{
			Error: nil,
			Network: NetworkState{
				Requests:       make(map[string]*RequestInfo),
				RateLimitWaits: make([]RateLimitWait, 0),
			},
		}

		clone := original.Clone()

		assert.Nil(t, clone.Error)
	})
}

func TestSnapshot_GetTrace(t *testing.T) {
	t.Run("returns existing trace", func(t *testing.T) {
		expectedState := &State{Type: Messages, Status: StatusCompleted}
		snapshot := &Snapshot{
			traces: map[ResourceType]*State{
				Messages: expectedState,
			},
		}

		state, exists := snapshot.GetTrace(Messages)

		assert.True(t, exists)
		require.NotNil(t, state)
		assert.Same(t, expectedState, state)
	})

	t.Run("returns false for non-existent trace", func(t *testing.T) {
		snapshot := &Snapshot{
			traces: map[ResourceType]*State{
				Messages: {Type: Messages},
			},
		}

		state, exists := snapshot.GetTrace(Todos)

		assert.False(t, exists)
		assert.Nil(t, state)
	})

	t.Run("returns false for empty traces map", func(t *testing.T) {
		snapshot := &Snapshot{
			traces: map[ResourceType]*State{},
		}

		state, exists := snapshot.GetTrace(Messages)

		assert.False(t, exists)
		assert.Nil(t, state)
	})
}

func TestSnapshot_GetElapsedTime(t *testing.T) {
	t.Run("returns duration since start time", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			startTime := time.Now().Add(-100 * time.Millisecond)
			snapshot := &Snapshot{startTime: startTime}

			elapsed := snapshot.GetElapsedTime()

			assert.Equal(t, 100*time.Millisecond, elapsed)
			assert.Less(t, elapsed, time.Second)
		})
	})

	t.Run("returns very small duration for recent start", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			snapshot := &Snapshot{startTime: time.Now()}

			elapsed := snapshot.GetElapsedTime()

			assert.Equal(t, time.Duration(0), elapsed)
		})
	})
}

func TestSnapshot_Traces(t *testing.T) {
	t.Run("returns all traces", func(t *testing.T) {
		traces := map[ResourceType]*State{
			Messages: {Type: Messages},
			Todos:    {Type: Todos},
			Comments: {Type: Comments},
		}
		snapshot := &Snapshot{traces: traces}

		result := snapshot.Traces()

		assert.Equal(t, traces, result)
		assert.Len(t, result, 3)
	})

	t.Run("returns empty map when no traces", func(t *testing.T) {
		snapshot := &Snapshot{traces: map[ResourceType]*State{}}

		result := snapshot.Traces()

		assert.Empty(t, result)
		assert.NotNil(t, result)
	})

	t.Run("returns same map reference", func(t *testing.T) {
		traces := map[ResourceType]*State{Messages: {Type: Messages}}
		snapshot := &Snapshot{traces: traces}

		result := snapshot.Traces()

		assert.Same(t, traces[Messages], result[Messages])
	})
}
