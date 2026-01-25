package progress

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDisplay(t *testing.T) {
	t.Run("creates display with default configuration", func(t *testing.T) {
		tracker := trace.NewTracer()
		d := NewDisplay(tracker)

		assert.NotNil(t, d)
		assert.Equal(t, tracker, d.tracker)
		assert.Equal(t, os.Stdout, d.writer)
		assert.Equal(t, defaultConfig, d.config)
		assert.NotNil(t, d.stopChan)
		assert.NotNil(t, d.doneChan)
		assert.NotNil(t, d.reqBuf)
		assert.Equal(t, 500, cap(d.reqBuf))
		assert.NotNil(t, d.caser)
	})

	t.Run("pre-allocates buffer", func(t *testing.T) {
		tracker := trace.NewTracer()
		d := NewDisplay(tracker)

		// buffer should be pre-grown
		assert.Equal(t, 8192, d.buf.Cap())
	})
}

func TestDisplay_Start(t *testing.T) {
	t.Run("starts update loop when enabled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, w := createTestDisplay(t, tracker)

			d.Start()
			defer d.Stop()

			synctest.Wait()
			assert.NotEmpty(t, w.String())
		})
	})

	t.Run("does nothing when already stopped", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, w := createTestDisplay(t, tracker)

		d.stopped = true
		d.Start()

		assert.True(t, d.stopped)
		assert.Empty(t, w.String())
	})

	t.Run("resets lastLines on start", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, _ := createTestDisplay(t, tracker)

			d.lastLines = 10

			d.Start()
			defer d.Stop()

			assert.Equal(t, 0, d.lastLines)
		})
	})
}

func TestDisplay_Stop(t *testing.T) {
	t.Run("stops running display", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, w := createTestDisplay(t, tracker)

			d.Start()
			d.Stop()

			assert.True(t, d.stopped)
			assert.Contains(t, w.String(), "Activity aggregation completed")
		})
	})

	t.Run("does nothing when already stopped", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, w := createTestDisplay(t, tracker)

			d.Start()
			d.Stop()
			assert.True(t, d.stopped)
			assert.Contains(t, w.String(), "Activity aggregation completed")

			d.Stop()
			assert.True(t, d.stopped)
			assert.Equal(t, 1, strings.Count(w.String(), "Activity aggregation completed"))
		})
	})
}

func TestDisplay_PrintError(t *testing.T) {
	t.Run("clears board and prints error", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, w := createTestDisplay(t, tracker)

		d.lastLines = 5

		// capture stderr
		oldStderr := os.Stderr
		r, stderr, _ := os.Pipe()
		os.Stderr = stderr

		d.PrintError("test error: %d", 42)

		stderr.Close()
		os.Stderr = oldStderr

		var stderrBuf bytes.Buffer
		io.Copy(&stderrBuf, r)

		output := w.String()
		assert.Contains(t, output, clearLine)
		assert.Contains(t, stderrBuf.String(), "ERROR: test error: 42")
		assert.Equal(t, 0, d.lastLines)
	})
}

func TestDisplay_PrintWarning(t *testing.T) {
	t.Run("clears board and prints warning", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, w := createTestDisplay(t, tracker)

		d.lastLines = 3

		d.PrintWarning("warning message: %v", true)

		output := w.String()
		assert.Contains(t, output, "WARNING: warning message: true")
		assert.Equal(t, 0, d.lastLines)
	})
}

func TestDisplay_updateLoop(t *testing.T) {
	t.Run("renders periodically until stopped", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, w := createTestDisplay(t, tracker)

			go d.updateLoop()

			synctest.Wait()
			initialOutput := w.String()
			assert.NotEmpty(t, initialOutput)

			// wait for another tick
			time.Sleep(d.config.refreshRate + 10*time.Millisecond)

			synctest.Wait()

			close(d.stopChan)
			<-d.doneChan

			finalOutput := w.String()
			assert.NotEqual(t, initialOutput, finalOutput)
		})
	})

	t.Run("closes doneChan on exit", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, _ := createTestDisplay(t, tracker)

			go d.updateLoop()
			time.Sleep(100 * time.Millisecond)

			close(d.stopChan)

			select {
			case <-d.doneChan:
				// success
			case <-time.After(1 * time.Second):
				t.Fatal("doneChan was not closed")
			}
		})
	})
}

func TestDisplay_render(t *testing.T) {
	t.Run("renders board when state exists", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, w := createTestDisplay(t, tracker)

		d.render()

		output := w.String()
		assert.Contains(t, output, "Fetching Activity Data")
		assert.Contains(t, output, "Elapsed:")
	})

	t.Run("advances spinner frame", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, _ := createTestDisplay(t, tracker)

		initialFrame := d.spinnerFrame

		d.render()

		assert.NotEqual(t, initialFrame, d.spinnerFrame)
		assert.Equal(t, (initialFrame+1)%len(spinnerFrames), d.spinnerFrame)
	})

	t.Run("updates lastLines", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, _ := createTestDisplay(t, tracker)

		d.render()

		assert.Equal(t, 3, d.lastLines)
	})
}

func TestDisplay_clearBoard(t *testing.T) {
	t.Run("does nothing when lastLines is zero", func(t *testing.T) {
		d := &Display{lastLines: 0}
		var buf bytes.Buffer

		d.clearBoard(&buf)

		assert.Empty(t, buf.String())
	})

	t.Run("clears single line", func(t *testing.T) {
		d := &Display{lastLines: 1}
		var buf bytes.Buffer

		d.clearBoard(&buf)

		assert.Equal(t, clearReturn, buf.String())
	})

	t.Run("clears multiple lines", func(t *testing.T) {
		d := &Display{lastLines: 3}
		var buf bytes.Buffer

		d.clearBoard(&buf)

		expected := clearAndUp + clearAndUp + clearReturn
		assert.Equal(t, expected, buf.String())
	})
}

func TestDisplay_drawBoard(t *testing.T) {
	t.Run("draws header with elapsed time", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, _ := createTestDisplay(t, tracker)
		var buf bytes.Buffer

		snapshot := tracker.Snapshot()
		d.drawBoard(&buf, snapshot)

		output := buf.String()
		assert.Contains(t, output, "Fetching Activity Data | Elapsed: ")
		assert.Contains(t, output, d.config.separator)
	})

	t.Run("draws traces for each recording type", func(t *testing.T) {
		tracker := trace.NewTracer()
		tracker.RegisterTrace(trace.Messages)
		tracker.StartSpan(trace.Messages, trace.SpanFetchData)
		d, _ := createTestDisplay(t, tracker)
		var buf bytes.Buffer

		snapshot := tracker.Snapshot()
		lineCount := d.drawBoard(&buf, snapshot)

		output := buf.String()
		assert.Contains(t, output, "Message")
		assert.Contains(t, output, "Fetching...")
		assert.Equal(t, 7, lineCount)
	})
}

func TestDisplay_writeRecordingTrace(t *testing.T) {
	t.Run("writes header with status icon", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, _ := createTestDisplay(t, tracker)
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		var buf bytes.Buffer

		lines := d.writeRecordingTrace(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "[~]")
		assert.Contains(t, output, "Message")
		assert.Contains(t, output, "Fetching...")
		assert.Equal(t, 4, lines)
	})
}

func TestDisplay_writeTraceContent(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("completed status", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusCompleted, trace.SpanFetchData)
		state.CompletionTime = time.Now()
		var buf bytes.Buffer

		lines := d.writeTraceContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Pages: 10")
		assert.Contains(t, output, "Fetched 100 resources from 10 pages")
		assert.Contains(t, output, "Filtered: 50 of 100 messages")
		assert.Contains(t, output, "✓ Completed")
		assert.Contains(t, output, "0 cached of 0 requests (0%)")
		assert.Equal(t, 4, lines)
	})

	t.Run("error status with error", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusError, trace.SpanFetchData)
		state.Error = errors.New("test error message")
		var buf bytes.Buffer

		lines := d.writeTraceContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "ERROR: test error message")
		assert.Equal(t, 1, lines)
	})

	t.Run("error status without error", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusError, trace.SpanFetchData)
		state.Error = nil
		var buf bytes.Buffer

		lines := d.writeTraceContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "ERROR: Unknown error")
		assert.Equal(t, 1, lines)
	})

	t.Run("running status", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		var buf bytes.Buffer

		lines := d.writeTraceContent(&buf, state)

		// writeRunningContent returns: writeLastPageInfo(1) + writeFetchInfo(1 + writeRequests(0)) = 2
		assert.Equal(t, 2, lines)
		assert.Contains(t, buf.String(), "Pages:")
		assert.Contains(t, buf.String(), "Fetching...")
	})

	t.Run("pending status returns zero lines", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusPending, trace.SpanFetchData)
		var buf bytes.Buffer

		lines := d.writeTraceContent(&buf, state)

		assert.Equal(t, 0, lines)
		assert.Empty(t, buf.String())
	})
}

func TestDisplay_writeRunningContent(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("SpanFindLastPage", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFindLastPage)
		var buf bytes.Buffer

		lines := d.writeRunningContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Probing last page...")
		assert.Equal(t, 1, lines)
		assert.Contains(t, output, spinnerFrames[d.spinnerFrame])
	})

	t.Run("SpanFetchData", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		var buf bytes.Buffer

		lines := d.writeRunningContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Pages:")
		assert.Contains(t, output, "Fetching...")
		// writeLastPageInfo(1) + writeFetchInfo(1 + writeRequests(0)) = 2
		assert.Equal(t, 2, lines)
	})

	t.Run("SpanFetchData with rate limit waits", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		state.Network.RateLimitWaits = []trace.RateLimitWait{
			{WaitDuration: 5 * time.Second, Reason: "429 Too Many Requests"},
		}
		var buf bytes.Buffer

		lines := d.writeRunningContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Pages:")
		assert.Contains(t, output, "Fetching...")
		assert.Contains(t, output, "Rate Limit")
		// writeLastPageInfo(1) + writeFetchInfo(1 + writeRequests(0) + writeRateLimitWaits(1)) = 1 + 2 = 3
		assert.Equal(t, 3, lines)
		assert.Contains(t, output, "429 Too Many Requests")
		assert.Contains(t, output, "Waited 5s")
		assert.Contains(t, output, "Total Waits: 1")
	})

	t.Run("SpanFilter", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFilter)
		var buf bytes.Buffer

		lines := d.writeRunningContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Pages:")
		assert.Contains(t, output, "Fetched")
		assert.Contains(t, output, "Filtered:")
		// writeLastPageInfo(1) + writeTotalFetchedInfo(1) + writeTotalFilteredInfo(1) = 3
		assert.Equal(t, 3, lines)
		assert.Contains(t, output, "Fetched 100 resources from 10 pages")
		assert.Contains(t, output, "Filtered: 50 of 100 messages")
	})

	t.Run("unknown span returns zero lines", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, "unknown_span")
		var buf bytes.Buffer

		lines := d.writeRunningContent(&buf, state)

		assert.Equal(t, 0, lines)
	})
}

func TestDisplay_writeErrorContent(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes error message", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusError, trace.SpanFetchData)
		state.Error = errors.New("connection timeout")
		var buf bytes.Buffer

		lines := d.writeErrorContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "ERROR: connection timeout")
		assert.Equal(t, 1, lines)
	})

	t.Run("writes unknown error when nil", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusError, trace.SpanFetchData)
		state.Error = nil
		var buf bytes.Buffer

		lines := d.writeErrorContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "ERROR: Unknown error")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_writeFindLastPageInfo(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes spinner and probing message", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFindLastPage)
		var buf bytes.Buffer

		lines := d.writeFindLastPageInfo(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Probing last page...")
		assert.Contains(t, output, spinnerFrames[d.spinnerFrame])
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_writeLastPageInfo(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes page count", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		state.Pagination.LastPageFound = 42
		var buf bytes.Buffer

		lines := d.writeLastPageInfo(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Pages: 42")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_writeFetchInfo(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes fetching status", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		var buf bytes.Buffer

		lines := d.writeFetchInfo(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Fetching...")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_writeTotalFetchedInfo(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes fetch summary", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFilter)
		state.Filter.TotalItems = 150
		state.Pagination.LastPageFound = 15
		var buf bytes.Buffer

		lines := d.writeTotalFetchedInfo(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Fetched 150 resources from 15 pages")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_writeTotalFilteredInfo(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes filter summary with pluralization", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFilter)
		state.Filter.TotalItemsAfterFilter = 25
		state.Filter.TotalItems = 100
		var buf bytes.Buffer

		lines := d.writeTotalFilteredInfo(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Filtered: 25 of 100 messages")
		assert.Equal(t, 1, lines)
	})

	t.Run("singular form for count of 1", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFilter)
		state.Filter.TotalItemsAfterFilter = 1
		state.Filter.TotalItems = 50
		var buf bytes.Buffer

		d.writeTotalFilteredInfo(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "1 of 50 message")
		assert.NotContains(t, output, "messages")
	})
}

func TestDisplay_writeCompletedContent(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes all completion info", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusCompleted, trace.SpanFetchData)
		state.CompletionTime = time.Now()
		var buf bytes.Buffer

		lines := d.writeCompletedContent(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "Pages:")
		assert.Contains(t, output, "Fetched")
		assert.Contains(t, output, "Filtered:")
		assert.Contains(t, output, "✓ Completed")
		assert.Equal(t, 4, lines)
	})
}

func TestDisplay_writeCompleteStats(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("with cached requests", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusCompleted, trace.SpanFetchData)
		state.StartTime = time.Now().Add(-10 * time.Second)
		state.CompletionTime = time.Now()
		state.Network.Requests["req1"] = &trace.RequestInfo{Cached: true, StatusCode: 200}
		state.Network.Requests["req2"] = &trace.RequestInfo{Cached: false, StatusCode: 200}
		state.Network.Requests["req3"] = &trace.RequestInfo{Cached: true, StatusCode: 200}
		var buf bytes.Buffer

		lines := d.writeCompleteStats(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "✓ Completed")
		assert.Contains(t, output, "2 cached of 3 requests")
		assert.Contains(t, output, "66%")
		assert.Equal(t, 1, lines)
	})

	t.Run("with no requests", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusCompleted, trace.SpanFetchData)
		state.StartTime = time.Now().Add(-5 * time.Second)
		state.CompletionTime = time.Now()
		var buf bytes.Buffer

		lines := d.writeCompleteStats(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "0 cached of 0 requests (0%)")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_writeRequests(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("no requests", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		var buf bytes.Buffer

		lines := d.writeRequests(&buf, state)

		assert.Equal(t, 0, lines)
	})

	t.Run("fewer requests than max", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		state.Network.Requests["req1"] = &trace.RequestInfo{
			URL:        "https://example.com/api/1",
			StatusCode: 200,
			EndTime:    time.Now(),
		}
		state.Network.Requests["req2"] = &trace.RequestInfo{
			URL:        "https://example.com/api/2",
			StatusCode: 304,
			Cached:     true,
			EndTime:    time.Now(),
		}
		var buf bytes.Buffer

		lines := d.writeRequests(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "└─")
		assert.Contains(t, output, "[200]")
		assert.Contains(t, output, "https://example.com/api/1")
		assert.Contains(t, output, "https://example.com/api/2")
		assert.Contains(t, output, "└─")
		assert.Contains(t, output, "[304]")
		assert.Contains(t, output, "(cached)")
		assert.Equal(t, 2, lines)
	})

	t.Run("more requests than max shows overflow", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		for i := range 10 {
			state.Network.Requests[string(rune('a'+i))] = &trace.RequestInfo{
				URL:        "https://example.com/api/" + string(rune('a'+i)),
				StatusCode: 200,
				EndTime:    time.Now().Add(time.Duration(i) * time.Second),
			}
		}
		var buf bytes.Buffer

		lines := d.writeRequests(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "└─")
		assert.Contains(t, output, "https://example.com/api/")
		assert.Contains(t, output, "[200]")
		assert.Contains(t, output, "... and")
		assert.Contains(t, output, " 7 more requests")
		// maxRequestsPerTrace (3) + 1 for overflow message
		assert.Equal(t, 4, lines)
	})

	t.Run("pending request without status code", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		state.Network.Requests["req1"] = &trace.RequestInfo{
			URL:        "https://example.com/api/pending",
			StatusCode: 0,
		}
		var buf bytes.Buffer

		lines := d.writeRequests(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "└─")
		assert.NotContains(t, output, "[0]")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_sortedRequests(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("completed requests come before pending", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		now := time.Now()
		state.Network.Requests["pending"] = &trace.RequestInfo{
			URL:        "https://example.com/pending",
			StatusCode: 0,
			EndTime:    now,
		}
		state.Network.Requests["completed"] = &trace.RequestInfo{
			URL:        "https://example.com/completed",
			StatusCode: 200,
			EndTime:    now.Add(-1 * time.Second),
		}

		sorted := d.sortedRequests(state)

		require.Len(t, sorted, 2)
		assert.Equal(t, 200, sorted[0].StatusCode)
		assert.Equal(t, 0, sorted[1].StatusCode)
	})

	t.Run("sorts by end time within same completion status", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		now := time.Now()
		state.Network.Requests["older"] = &trace.RequestInfo{
			URL:        "https://example.com/older",
			StatusCode: 200,
			EndTime:    now.Add(-2 * time.Second),
		}
		state.Network.Requests["newer"] = &trace.RequestInfo{
			URL:        "https://example.com/newer",
			StatusCode: 200,
			EndTime:    now,
		}

		sorted := d.sortedRequests(state)

		require.Len(t, sorted, 2)
		assert.Contains(t, sorted[0].URL, "newer")
		assert.Contains(t, sorted[1].URL, "older")
	})

	t.Run("reuses buffer across calls", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		state.Network.Requests["req1"] = &trace.RequestInfo{URL: "url1", StatusCode: 200}

		d.sortedRequests(state)
		bufPtr1 := &d.reqBuf[0]

		state.Network.Requests["req2"] = &trace.RequestInfo{URL: "url2", StatusCode: 200}
		d.sortedRequests(state)

		// buffer should be reused (same underlying array within capacity)
		assert.Equal(t, 2, len(d.reqBuf))
		assert.Equal(t, bufPtr1, &d.reqBuf[0])
	})
}

func TestDisplay_writeRateLimitWaits(t *testing.T) {
	tracker := trace.NewTracer()
	d, _ := createTestDisplay(t, tracker)

	t.Run("writes rate limit info", func(t *testing.T) {
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)
		state.Network.RateLimitWaits = []trace.RateLimitWait{
			{WaitDuration: 1 * time.Second, Reason: "first"},
			{WaitDuration: 5 * time.Second, Reason: "429 response"},
		}
		var buf bytes.Buffer

		lines := d.writeRateLimitWaits(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "[Rate Limit] Waited 5s")
		assert.Contains(t, output, "Reason: 429 response")
		assert.Contains(t, output, "Total Waits: 2")
		assert.Equal(t, 1, lines)
	})
}

func TestDisplay_printFinalSummary(t *testing.T) {
	t.Run("prints summary when state exists", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, w := createTestDisplay(t, tracker)

		d.printFinalSummary()

		output := w.String()
		assert.Contains(t, output, "Activity aggregation completed")
		assert.Contains(t, output, d.config.separator)
	})
}

func TestCalculateElapsed(t *testing.T) {
	t.Run("uses completion time when available", func(t *testing.T) {
		state := &trace.State{
			StartTime:      time.Now().Add(-10 * time.Second),
			CompletionTime: time.Now().Add(-5 * time.Second),
		}
		elapsed := calculateElapsed(state)

		assert.Equal(t, 5*time.Second, elapsed)
	})

	t.Run("uses time.Since when completion time is zero", func(t *testing.T) {
		state := &trace.State{
			StartTime:      time.Now().Add(-3 * time.Second),
			CompletionTime: time.Time{},
		}

		elapsed := calculateElapsed(state)

		assert.Equal(t, 3*time.Second, elapsed)
	})
}

func TestCountCachedRequests(t *testing.T) {
	t.Run("counts cached and total correctly", func(t *testing.T) {
		state := &trace.State{
			Network: trace.NetworkState{
				Requests: map[string]*trace.RequestInfo{
					"req1": {Cached: true, StatusCode: 200},
					"req2": {Cached: false, StatusCode: 200},
					"req3": {Cached: true, StatusCode: 200},
					"req4": {Cached: false, StatusCode: 200},
					"req5": {Cached: true, StatusCode: 200},
				},
			},
		}

		cached, total := countCachedRequests(state)

		assert.Equal(t, 3, cached)
		assert.Equal(t, 5, total)
	})

	t.Run("handles empty requests", func(t *testing.T) {
		state := &trace.State{
			Network: trace.NetworkState{
				Requests: map[string]*trace.RequestInfo{},
			},
		}

		cached, total := countCachedRequests(state)

		assert.Equal(t, 0, cached)
		assert.Equal(t, 0, total)
	})
}

func TestDisplay_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent render and stop", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, w := createTestDisplay(t, tracker)

			d.Start()
			defer d.Stop()

			for i := range 10 {
				go func() {
					d.PrintWarning("test %d", i)
				}()
			}

			synctest.Wait()

			assert.False(t, d.stopped)
			assert.Contains(t, w.String(), "Fetching Activity Data")
			for i := range 10 {
				assert.Contains(t, w.String(), "WARNING: test "+strconv.Itoa(i))
			}
		})
	})

	t.Run("handles concurrent error printing", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tracker := trace.NewTracer()
			d, _ := createTestDisplay(t, tracker)

			// capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			d.Start()
			defer d.Stop()

			for i := range 5 {
				go func() {
					d.PrintError("error %d", i)
				}()
			}

			synctest.Wait()

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			assert.False(t, d.stopped)
			for i := range 5 {
				assert.Contains(t, buf.String(), "ERROR: error "+strconv.Itoa(i))
			}
		})
	})
}

func TestURLShortening(t *testing.T) {
	t.Run("long URLs are shortened in request display", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, _ := createTestDisplay(t, tracker)
		state := setupTraceState(trace.Messages, trace.StatusRunning, trace.SpanFetchData)

		longURL := "https://example.com/very/long/path/that/exceeds/the/maximum/length/limit"
		state.Network.Requests["req1"] = &trace.RequestInfo{
			URL:        longURL,
			StatusCode: 200,
		}
		var buf bytes.Buffer

		d.writeRequests(&buf, state)

		output := buf.String()
		assert.Contains(t, output, "...")
		firstLine := strings.Split(output, "\n")[0]
		// URL should be truncated from the beginning (total 50 chars shown including ellipsis)
		assert.Contains(t, firstLine, "...long/path/that/exceeds/the/maximum/length/limit")
	})
}

func TestSpinnerFrameWrapping(t *testing.T) {
	t.Run("spinner frame wraps around", func(t *testing.T) {
		tracker := trace.NewTracer()
		d, _ := createTestDisplay(t, tracker)

		for i := 1; i < len(spinnerFrames); i++ {
			d.render()
			assert.Equal(t, i, d.spinnerFrame)
		}
	})
}

// testWriter captures output for testing
type testWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *testWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *testWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Reset()
}

// createTestDisplay creates a Display with a custom writer for testing
func createTestDisplay(t *testing.T, tracker *trace.Tracer) (*Display, *testWriter) {
	t.Helper()
	d := NewDisplay(tracker)
	w := &testWriter{}
	d.writer = w
	return d, w
}

// setupTraceState creates a TraceState with common test data
func setupTraceState(
	resourceType trace.ResourceType,
	status trace.Status,
	span trace.Span,
) *trace.State {
	return &trace.State{
		Type:        resourceType,
		Status:      status,
		CurrentSpan: span,
		StartTime:   time.Now().Add(-5 * time.Second),
		Pagination: trace.PaginationState{
			LastPageFound: 10,
		},
		Filter: trace.FilterState{
			TotalItems:            100,
			TotalItemsAfterFilter: 50,
		},
		Network: trace.NetworkState{
			Requests: make(map[string]*trace.RequestInfo),
		},
	}
}
