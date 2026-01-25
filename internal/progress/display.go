package progress

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/trace"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Display handles rendering the progress status board
type Display struct {
	tracker      *trace.Tracer
	writer       io.Writer
	config       config
	mu           sync.Mutex
	lastLines    int
	spinnerFrame int
	stopped      bool
	stopChan     chan struct{}
	doneChan     chan struct{}

	buf    bytes.Buffer         // reusable buffer for rendering
	reqBuf []*trace.RequestInfo // reusable scratchpad for request sorting
	caser  cases.Caser          // pre-initialized caser
}

func NewDisplay(tracker *trace.Tracer) *Display {
	d := &Display{
		tracker:  tracker,
		writer:   os.Stdout,
		config:   defaultConfig,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		// pre-allocating for 1000 requests per resourceType prevents resizing during
		// standard load. Trades some memory for CPU efficiency.
		reqBuf: make([]*trace.RequestInfo, 0, 500),
		caser:  cases.Title(language.English),
	}

	d.buf.Grow(8192) // pre-allocate larger buffer for concurrent display

	return d
}

// Start begins the display update loop.
// safe to call multiple times; subsequent calls after Stop are ignored.
func (d *Display) Start() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.lastLines = 0
	d.mu.Unlock()

	go d.updateLoop()
}

// Stop gracefully stops the display and shows the final summary.
// safe to call multiple times; only the first call has effect.
func (d *Display) Stop() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.stopped = true
	d.mu.Unlock()

	close(d.stopChan)
	<-d.doneChan

	d.printFinalSummary()
}

// PrintError prints an error message that persists above the status board
func (d *Display) PrintError(format string, args ...any) {
	// clear the board while holding the lock
	d.mu.Lock()
	d.buf.Reset()
	d.clearBoard(&d.buf)
	_, _ = d.writer.Write(d.buf.Bytes())
	d.lastLines = 0
	d.mu.Unlock()

	// write to stderr outside the lock to avoid blocking other renders
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
}

// PrintWarning prints a warning message that persists above the status board.
func (d *Display) PrintWarning(format string, args ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.buf.Reset()
	d.clearBoard(&d.buf)
	fmt.Fprintf(&d.buf, "WARNING: "+format+"\n", args...)
	_, _ = d.writer.Write(d.buf.Bytes())
	d.lastLines = 0
}

// updateLoop periodically refreshes the display
func (d *Display) updateLoop() {
	defer close(d.doneChan)

	d.render()

	ticker := time.NewTicker(d.config.refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.render()
		case <-d.stopChan:
			d.render()
			return
		}
	}
}

// render draws the current status board to the terminal
func (d *Display) render() {
	snapshot := d.tracker.Snapshot()

	d.mu.Lock()
	defer d.mu.Unlock()

	d.buf.Reset()
	d.clearBoard(&d.buf)
	lines := d.drawBoard(&d.buf, snapshot)
	d.writer.Write(d.buf.Bytes())

	d.lastLines = lines
	d.spinnerFrame = (d.spinnerFrame + 1) % len(spinnerFrames)
}

// clearBoard writes ANSI codes to clear the previous board from the terminal
func (d *Display) clearBoard(buf *bytes.Buffer) {
	if d.lastLines == 0 {
		return
	}

	for i := 0; i < d.lastLines-1; i++ {
		buf.WriteString(clearAndUp)
	}
	buf.WriteString(clearReturn)
}

// drawBoard renders the status board into the buffer
func (d *Display) drawBoard(buf *bytes.Buffer, snapshot *trace.Snapshot) int {
	lineCount := 0

	// Header
	elapsed := d.tracker.GetElapsedTime().Round(time.Second)

	buf.WriteString(d.config.separator)
	buf.WriteString("\nFetching Activity Data | Elapsed: ")
	buf.WriteString(elapsed.String())
	buf.WriteByte('\n')
	buf.WriteString(d.config.separator)
	lineCount += 2

	for _, rt := range trace.ResourceTypes {
		if tracer, exists := snapshot.GetTrace(rt); exists {
			lineCount += d.writeRecordingTrace(buf, tracer)
		}
	}

	buf.WriteString(d.config.separator)
	return lineCount + 1
}

// writeRecordingTrace renders a single recording type's trace section.
func (d *Display) writeRecordingTrace(buf *bytes.Buffer, state *trace.State) int {
	lines := 2

	// Header with status icon and entity name
	buf.WriteByte('\n')
	buf.WriteString(statusIcon(state.Status))
	buf.WriteByte(' ')
	buf.WriteString(d.caser.String(entityName(state.Type, 0)))
	buf.WriteByte('\n')

	lines += d.writeTraceContent(buf, state)
	return lines
}

// writeTraceContent delegates to the appropriate writer based on trace status.
func (d *Display) writeTraceContent(buf *bytes.Buffer, state *trace.State) int {
	switch state.Status {
	case trace.StatusCompleted:
		return d.writeCompletedContent(buf, state)
	case trace.StatusError:
		return d.writeErrorContent(buf, state)
	case trace.StatusRunning:
		return d.writeRunningContent(buf, state)
	default:
		return 0
	}
}

// writeCompletedContent writes the summary for a completed state.
func (d *Display) writeCompletedContent(buf *bytes.Buffer, state *trace.State) int {
	lines := d.writeLastPageInfo(buf, state)
	lines += d.writeTotalFetchedInfo(buf, state)
	lines += d.writeTotalFilteredInfo(buf, state)
	lines += d.writeCompleteStats(buf, state)
	return lines
}

// writeRunningContent writes progress information for an active state.
func (d *Display) writeRunningContent(buf *bytes.Buffer, state *trace.State) int {
	switch state.CurrentSpan {
	case trace.SpanFindLastPage:
		return d.writeFindLastPageInfo(buf, state)
	case trace.SpanFetchData:
		lines := d.writeLastPageInfo(buf, state)
		lines += d.writeFetchInfo(buf, state)
		return lines
	case trace.SpanFilter:
		lines := d.writeLastPageInfo(buf, state)
		lines += d.writeTotalFetchedInfo(buf, state)
		lines += d.writeTotalFilteredInfo(buf, state)
		return lines
	default:
		return 0
	}
}

// writeErrorContent writes error information for a failed state.
func (d *Display) writeErrorContent(buf *bytes.Buffer, state *trace.State) int {
	buf.WriteString("    ERROR: ")
	if state.Error != nil {
		buf.WriteString(state.Error.Error())
	} else {
		buf.WriteString("Unknown error")
	}
	buf.WriteByte('\n')
	return 1
}

func (d *Display) writeFindLastPageInfo(buf *bytes.Buffer, state *trace.State) int {
	buf.WriteString(spinnerFrames[d.spinnerFrame])
	buf.WriteString("   Probing last page...\n")
	return 1 + d.writeRequests(buf, state)
}

func (d *Display) writeLastPageInfo(buf *bytes.Buffer, state *trace.State) int {
	buf.WriteString("    Pages: ")
	buf.WriteString(strconv.Itoa(state.Pagination.LastPageFound))
	buf.WriteByte('\n')
	return 1
}

func (d *Display) writeFetchInfo(buf *bytes.Buffer, state *trace.State) int {
	buf.WriteString(spinnerFrames[d.spinnerFrame])
	buf.WriteString("   Fetching...\n")

	lines := 1 + d.writeRequests(buf, state)

	if len(state.Network.RateLimitWaits) > 0 {
		lines += d.writeRateLimitWaits(buf, state)
	}
	return lines
}

func (d *Display) writeTotalFetchedInfo(buf *bytes.Buffer, state *trace.State) int {
	buf.WriteString("    Fetched ")
	buf.WriteString(strconv.Itoa(state.Filter.TotalItems))
	buf.WriteString(" resources from ")
	buf.WriteString(strconv.Itoa(state.Pagination.LastPageFound))
	buf.WriteString(" pages\n")
	return 1
}

func (d *Display) writeTotalFilteredInfo(buf *bytes.Buffer, state *trace.State) int {
	buf.WriteString("    Filtered: ")
	buf.WriteString(strconv.Itoa(state.Filter.TotalItemsAfterFilter))
	buf.WriteString(" of ")
	buf.WriteString(strconv.Itoa(state.Filter.TotalItems))
	buf.WriteByte(' ')
	buf.WriteString(entityName(state.Type, state.Filter.TotalItemsAfterFilter))
	buf.WriteByte('\n')
	return 1
}

func (d *Display) writeCompleteStats(buf *bytes.Buffer, state *trace.State) int {
	elapsed := calculateElapsed(state)
	cached, total := countCachedRequests(state)

	cachePercent := 0
	if total > 0 {
		cachePercent = (cached * 100) / total
	}

	buf.WriteString("    ✓ Completed | ")
	buf.WriteString(strconv.Itoa(cached))
	buf.WriteString(" cached of ")
	buf.WriteString(strconv.Itoa(total))
	buf.WriteString(" requests (")
	buf.WriteString(strconv.Itoa(cachePercent))
	buf.WriteString("%) | ")
	buf.WriteString(elapsed.String())
	buf.WriteByte('\n')
	return 1
}

func (d *Display) writeRequests(buf *bytes.Buffer, state *trace.State) int {
	requests := d.sortedRequests(state)
	maxDisplayed := min(len(requests), d.config.maxRequestsPerTrace)

	lines := 0
	for i := range maxDisplayed {
		req := requests[i]
		buf.WriteString("    └─ ")
		buf.WriteString(shortenURL(req.URL, d.config.maxURLLength))
		if req.StatusCode != 0 {
			buf.WriteString(" [")
			buf.WriteString(strconv.Itoa(req.StatusCode))
			buf.WriteByte(']')
		}
		if req.Cached {
			buf.WriteString(" (cached)")
		}
		buf.WriteByte('\n')
		lines++
	}

	// show overflow indicator if there are more requests
	if remaining := state.Network.CompletedRequests() - maxDisplayed; remaining > 0 {
		buf.WriteString("    └─ ... and ")
		buf.WriteString(strconv.Itoa(remaining))
		buf.WriteString(" more requests\n")
		lines++
	}

	return lines
}

// sortedRequests returns requests sorted for display:
// completed requests first (by most recent end time), then pending requests.
func (d *Display) sortedRequests(state *trace.State) []*trace.RequestInfo {
	// reset length, keep capacity to avoid reallocations
	d.reqBuf = d.reqBuf[:0]

	for _, req := range state.Network.Requests {
		d.reqBuf = append(d.reqBuf, req)
	}

	sort.Slice(d.reqBuf, func(i, j int) bool {
		iCompleted := d.reqBuf[i].StatusCode != 0
		jCompleted := d.reqBuf[j].StatusCode != 0

		// Prioritize completed requests over pending ones
		if iCompleted != jCompleted {
			return iCompleted
		}
		// Within same completion status, sort by end time (most recent first)
		return d.reqBuf[i].EndTime.After(d.reqBuf[j].EndTime)
	})

	return d.reqBuf
}

func (d *Display) writeRateLimitWaits(buf *bytes.Buffer, state *trace.State) int {
	lastWait := state.Network.RateLimitWaits[len(state.Network.RateLimitWaits)-1]
	buf.WriteString("    └─ [Rate Limit] Waited ")
	buf.WriteString((lastWait.WaitDuration).String())
	buf.WriteString(", Reason: ")
	buf.WriteString(lastWait.Reason)
	buf.WriteString(", Total Waits: ")
	buf.WriteString(strconv.Itoa(len(state.Network.RateLimitWaits)))
	buf.WriteByte('\n')
	return 1
}

func (d *Display) printFinalSummary() {
	elapsed := d.tracker.GetElapsedTime().Round(time.Second)
	fmt.Fprintf(d.writer, "\nActivity aggregation completed in %s\n%s\n", elapsed, d.config.separator)
}

// calculateElapsed returns the elapsed time for a trace, using completion time if available.
func calculateElapsed(state *trace.State) time.Duration {
	if !state.CompletionTime.IsZero() {
		return state.CompletionTime.Sub(state.StartTime).Round(100 * time.Millisecond)
	}
	return time.Since(state.StartTime).Round(100 * time.Millisecond)
}

// countCachedRequests returns the count of cached requests and total requests.
func countCachedRequests(state *trace.State) (cached, total int) {
	for _, req := range state.Network.Requests {
		if req.Cached {
			cached++
		}
	}
	return cached, state.Network.CompletedRequests()
}
