package trace

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTracer(t *testing.T) {
	t.Run("creates tracer with initialized fields", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()

			require.NotNil(t, tracer)
			assert.NotNil(t, tracer.traces)
			assert.Empty(t, tracer.traces)
			assert.Equal(t, time.Now().UnixNano(), tracer.startTime.UnixNano())
		})
	})
}

func TestTracer_RegisterTrace(t *testing.T) {
	t.Run("registers new trace with initial state", func(t *testing.T) {
		tracer := NewTracer()

		tracer.RegisterTrace(Messages)

		snapshot := tracer.Snapshot()
		state, exists := snapshot.GetTrace(Messages)
		require.True(t, exists)

		assert.Equal(t, Messages, state.Type)
		assert.Equal(t, StatusPending, state.Status)
		assert.False(t, state.StartTime.IsZero())
		assert.NotNil(t, state.Network.Requests)
		assert.NotNil(t, state.Network.RateLimitWaits)
		assert.Empty(t, state.Network.Requests)
	})

	t.Run("overwrites existing trace", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)
		tracer.SetLastPageFound(Messages, 10)
		tracer.SetTotalItems(Messages, 100)

		tracer.RegisterTrace(Messages)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, 0, state.Pagination.LastPageFound)
		assert.Equal(t, 0, state.Filter.TotalItems)
	})

	t.Run("registers multiple different traces", func(t *testing.T) {
		tracer := NewTracer()

		tracer.RegisterTrace(Messages)
		tracer.RegisterTrace(Todos)
		tracer.RegisterTrace(Comments)

		snapshot := tracer.Snapshot()
		assert.Len(t, snapshot.Traces(), 3)
	})
}

func TestTracer_StartSpan(t *testing.T) {
	t.Run("starts span on registered trace", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.StartSpan(Messages, SpanFetchData)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, StatusRunning, state.Status)
		assert.Equal(t, SpanFetchData, state.CurrentSpan)
		assert.False(t, state.SpanStartTime.IsZero())
	})

	t.Run("updates span on already running trace", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)
			tracer.StartSpan(Messages, SpanFindLastPage)
			firstSpanStart := tracer.Snapshot().traces[Messages].SpanStartTime

			time.Sleep(100 * time.Millisecond)
			tracer.StartSpan(Messages, SpanFetchData)

			snapshot := tracer.Snapshot()
			state, _ := snapshot.GetTrace(Messages)
			assert.Equal(t, SpanFetchData, state.CurrentSpan)
			assert.Equal(t, firstSpanStart.Add(100*time.Millisecond), state.SpanStartTime)
		})
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.StartSpan(Messages, SpanFetchData)

		snapshot := tracer.Snapshot()
		_, exists := snapshot.GetTrace(Messages)
		assert.False(t, exists)
	})
}

func TestTracer_CompleteSpan(t *testing.T) {
	t.Run("sets span end time", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)
			tracer.StartSpan(Messages, SpanFetchData)

			time.Sleep(100 * time.Millisecond)

			tracer.CompleteSpan(Messages)

			snapshot := tracer.Snapshot()
			state, _ := snapshot.GetTrace(Messages)
			assert.Equal(t, state.SpanStartTime.Add(100*time.Millisecond), state.SpanEndTime)
		})
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.CompleteSpan(Messages)

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_CompleteTrace(t *testing.T) {
	t.Run("marks trace as completed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)

			time.Sleep(100 * time.Millisecond)

			tracer.CompleteTrace(Messages)

			snapshot := tracer.Snapshot()
			state, _ := snapshot.GetTrace(Messages)
			assert.Equal(t, StatusCompleted, state.Status)
			assert.Equal(t, state.StartTime.Add(100*time.Millisecond), state.CompletionTime)
		})
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.CompleteTrace(Messages)

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_FailTrace(t *testing.T) {
	t.Run("marks trace as failed with error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)
			testErr := errors.New("something went wrong")

			time.Sleep(100 * time.Millisecond)
			tracer.FailTrace(Messages, testErr)

			snapshot := tracer.Snapshot()
			state, _ := snapshot.GetTrace(Messages)
			assert.Equal(t, StatusError, state.Status)
			assert.Equal(t, testErr, state.Error)
			assert.Equal(t, state.StartTime.Add(100*time.Millisecond), state.CompletionTime)
		})
	})

	t.Run("handles nil error", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.FailTrace(Messages, nil)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, StatusError, state.Status)
		assert.Nil(t, state.Error)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.FailTrace(Messages, errors.New("error"))

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_SetLastPageFound(t *testing.T) {
	t.Run("sets last page value", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.SetLastPageFound(Messages, 42)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, 42, state.Pagination.LastPageFound)
	})

	t.Run("updates existing value", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)
		tracer.SetLastPageFound(Messages, 10)

		tracer.SetLastPageFound(Messages, 20)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, 20, state.Pagination.LastPageFound)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.SetLastPageFound(Messages, 42)

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_SetTotalItems(t *testing.T) {
	t.Run("sets total items count", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.SetTotalItems(Messages, 100)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, 100, state.Filter.TotalItems)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.SetTotalItems(Messages, 100)

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_SetTotalItemsAfterFilter(t *testing.T) {
	t.Run("sets filtered items count", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.SetTotalItemsAfterFilter(Messages, 75)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, 75, state.Filter.TotalItemsAfterFilter)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.SetTotalItemsAfterFilter(Messages, 75)

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_StartRequest(t *testing.T) {
	t.Run("records new request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)
			url := "https://api.example.com/messages"

			tracer.StartRequest(Messages, url)

			snapshot := tracer.Snapshot()
			state, _ := snapshot.GetTrace(Messages)
			require.Contains(t, state.Network.Requests, url)
			req := state.Network.Requests[url]
			assert.Equal(t, url, req.URL)
			assert.Equal(t, 0, req.StatusCode)
			assert.False(t, req.Cached)
			assert.Equal(t, time.Now(), req.StartTime)
		})
	})

	t.Run("overwrites existing request", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)
		url := "https://api.example.com/messages"
		tracer.StartRequest(Messages, url)
		tracer.CompleteRequest(Messages, url, 200)

		tracer.StartRequest(Messages, url)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		req := state.Network.Requests[url]
		assert.Equal(t, 0, req.StatusCode)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.StartRequest(Messages, "https://example.com")

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_CompleteRequest(t *testing.T) {
	t.Run("completes request with status code", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)
			url := "https://api.example.com/messages"
			tracer.StartRequest(Messages, url)

			time.Sleep(100 * time.Millisecond)
			tracer.CompleteRequest(Messages, url, 200)

			snapshot := tracer.Snapshot()
			state, _ := snapshot.GetTrace(Messages)
			req := state.Network.Requests[url]
			assert.Equal(t, 200, req.StatusCode)
			assert.False(t, req.Cached)
			assert.Equal(t, req.StartTime.Add(100*time.Millisecond), req.EndTime)
		})
	})

	t.Run("marks 304 response as cached", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)
		url := "https://api.example.com/messages"
		tracer.StartRequest(Messages, url)

		tracer.CompleteRequest(Messages, url, 304)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		req := state.Network.Requests[url]
		assert.Equal(t, 304, req.StatusCode)
		assert.True(t, req.Cached)
	})

	t.Run("non-304 response is not cached", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)
		url := "https://api.example.com/messages"
		tracer.StartRequest(Messages, url)

		tracer.CompleteRequest(Messages, url, 200)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.False(t, state.Network.Requests[url].Cached)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.CompleteRequest(Messages, "https://example.com", 200)

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})

	t.Run("no-op on non-existent request", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.CompleteRequest(Messages, "https://unknown.com", 200)

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		assert.Empty(t, state.Network.Requests)
	})
}

func TestTracer_RecordRateLimitWait(t *testing.T) {
	t.Run("records rate limit wait", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.RecordRateLimitWait(Messages, 5*time.Second, "429 Too Many Requests")

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		require.Len(t, state.Network.RateLimitWaits, 1)
		assert.Equal(t, 5*time.Second, state.Network.RateLimitWaits[0].WaitDuration)
		assert.Equal(t, "429 Too Many Requests", state.Network.RateLimitWaits[0].Reason)
	})

	t.Run("appends multiple rate limit waits", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)

		tracer.RecordRateLimitWait(Messages, time.Second, "first wait")
		tracer.RecordRateLimitWait(Messages, 2*time.Second, "second wait")
		tracer.RecordRateLimitWait(Messages, 3*time.Second, "third wait")

		snapshot := tracer.Snapshot()
		state, _ := snapshot.GetTrace(Messages)
		require.Len(t, state.Network.RateLimitWaits, 3)
		assert.Equal(t, "first wait", state.Network.RateLimitWaits[0].Reason)
		assert.Equal(t, "second wait", state.Network.RateLimitWaits[1].Reason)
		assert.Equal(t, "third wait", state.Network.RateLimitWaits[2].Reason)
	})

	t.Run("no-op on non-existent trace", func(t *testing.T) {
		tracer := NewTracer()

		tracer.RecordRateLimitWait(Messages, time.Second, "wait")

		snapshot := tracer.Snapshot()
		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_Snapshot(t *testing.T) {
	t.Run("creates immutable snapshot", func(t *testing.T) {
		tracer := NewTracer()
		tracer.RegisterTrace(Messages)
		tracer.SetTotalItems(Messages, 100)

		snapshot := tracer.Snapshot()

		// modify tracer after snapshot
		tracer.SetTotalItems(Messages, 200)
		tracer.RegisterTrace(Todos)

		state, _ := snapshot.GetTrace(Messages)
		assert.Equal(t, 100, state.Filter.TotalItems)

		_, todoExists := snapshot.GetTrace(Todos)
		assert.False(t, todoExists)
	})

	t.Run("includes start time", func(t *testing.T) {
		tracer := NewTracer()

		snapshot := tracer.Snapshot()

		assert.Equal(t, tracer.startTime, snapshot.startTime)
	})

	t.Run("deep copies all traces", func(t *testing.T) {
		tracer := NewTracer()
		for _, rt := range ResourceTypes {
			tracer.RegisterTrace(rt)
		}

		snapshot := tracer.Snapshot()

		assert.Len(t, snapshot.Traces(), len(ResourceTypes))
		for _, rt := range ResourceTypes {
			state, exists := snapshot.GetTrace(rt)
			assert.True(t, exists)
			assert.Equal(t, rt, state.Type)
			assert.NotSame(t, tracer.traces[rt], state)
		}
	})

	t.Run("returns empty snapshot for new tracer", func(t *testing.T) {
		tracer := NewTracer()

		snapshot := tracer.Snapshot()

		assert.Empty(t, snapshot.Traces())
	})
}

func TestTracer_GetElapsedTime(t *testing.T) {
	t.Run("returns duration since creation", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			time.Sleep(100 * time.Millisecond)

			elapsed := tracer.GetElapsedTime()

			assert.Equal(t, 100*time.Millisecond, elapsed)
		})
	})

	t.Run("increases over time", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()

			first := tracer.GetElapsedTime()
			time.Sleep(5 * time.Millisecond)
			second := tracer.GetElapsedTime()

			assert.Greater(t, second, first)
		})
	})
}

func TestTracer_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent operations safely", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			const goroutines = 100
			const operationsPerGoroutine = 50

			for i := range goroutines {
				go func(id int) {
					rt := ResourceTypes[id%len(ResourceTypes)]

					for j := range operationsPerGoroutine {
						tracer.RegisterTrace(rt)
						tracer.StartSpan(rt, SpanFetchData)
						tracer.StartRequest(rt, "http://example.com/page")
						tracer.CompleteRequest(rt, "http://example.com/page", 200)
						tracer.RecordRateLimitWait(rt, 100*time.Millisecond, "rate limit")
						tracer.SetTotalItems(rt, j)
						tracer.SetTotalItemsAfterFilter(rt, j/2)
						tracer.SetLastPageFound(rt, j)
						tracer.CompleteSpan(rt)
						_ = tracer.Snapshot()
						_ = tracer.GetElapsedTime()
					}
					tracer.CompleteTrace(rt)
				}(i)
			}

			synctest.Wait()

			// verify tracer is functional after concurrent access
			snapshot := tracer.Snapshot()
			assert.NotEmpty(t, snapshot.Traces())
		})
	})

	t.Run("concurrent snapshots are independent", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracer := NewTracer()
			tracer.RegisterTrace(Messages)
			tracer.SetTotalItems(Messages, 50)

			snapshots := make([]*Snapshot, 10)

			for i := range 10 {
				go func(idx int) {
					snapshots[idx] = tracer.Snapshot()
				}(i)
			}

			synctest.Wait()

			// all snapshots should be valid and have the same data
			for i, snapshot := range snapshots {
				state, exists := snapshot.GetTrace(Messages)
				require.True(t, exists, "snapshot %d should have Messages trace", i)
				assert.Equal(t, 50, state.Filter.TotalItems)
				assert.NotSame(t, tracer.traces[Messages], state, "snapshot %d state should be a copy", i)
			}
		})
	})
}
