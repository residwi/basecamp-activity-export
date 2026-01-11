package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/residwi/bcreview/internal/tracking"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Display handles rendering the progress status board
type Display struct {
	tracker      *tracking.Tracker
	writer       io.Writer
	config       DisplayConfig
	mu           sync.Mutex
	lastLines    int
	spinnerFrame int
	stopChan     chan struct{}
	doneChan     chan struct{}
	buf          bytes.Buffer // Reusable buffer for rendering
}

func NewDisplay(tracker *tracking.Tracker) *Display {
	d := &Display{
		tracker:  tracker,
		writer:   os.Stdout,
		config:   DefaultDisplayConfig,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}

	d.buf.Grow(8192) // Pre-allocate larger buffer for concurrent display

	return d
}

// Start begins the display update loop
func (d *Display) Start() {
	if !d.tracker.IsEnabled() {
		return
	}

	select {
	case <-d.stopChan:
		return
	default:
	}

	d.mu.Lock()
	d.lastLines = 0
	d.mu.Unlock()

	go d.updateLoop()
}

// Stop gracefully stops the display and shows final state
func (d *Display) Stop() {
	if !d.tracker.IsEnabled() {
		return
	}

	select {
	case <-d.stopChan:
		return
	default:
		close(d.stopChan)
	}

	<-d.doneChan

	d.printFinalSummary()
}

// PrintError prints an error message that persists above the status board
func (d *Display) PrintError(format string, args ...any) {
	if !d.tracker.IsEnabled() {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.buf.Reset()
	d.clearBoard(&d.buf)
	d.writer.Write(d.buf.Bytes())

	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	d.lastLines = 0
}

// PrintWarning prints a warning message that persists
func (d *Display) PrintWarning(format string, args ...any) {
	if !d.tracker.IsEnabled() {
		fmt.Fprintf(os.Stderr, "WARNING: "+format+"\n", args...)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.buf.Reset()
	d.clearBoard(&d.buf)

	fmt.Fprintf(&d.buf, "WARNING: "+format+"\n", args...)
	d.writer.Write(d.buf.Bytes())

	d.lastLines = 0
}

// updateLoop periodically refreshes the display
func (d *Display) updateLoop() {
	defer close(d.doneChan)

	d.render()

	ticker := time.NewTicker(100 * time.Millisecond)
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

// render draws the current status board
func (d *Display) render() {
	state := d.tracker.GetState()
	if state == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.buf.Reset()
	d.clearBoard(&d.buf)

	lines := d.drawBoard(&d.buf, state)

	d.writer.Write(d.buf.Bytes())

	d.lastLines = lines
	d.spinnerFrame = (d.spinnerFrame + 1) % len(SpinnerFrames)
}

// clearBoard clears the previous status board from the terminal
func (d *Display) clearBoard(buf *bytes.Buffer) {
	if d.lastLines == 0 {
		return
	}

	clearAndUp := "\r" + ClearLine + "\033[1A"
	for i := 0; i < d.lastLines-1; i++ {
		buf.WriteString(clearAndUp)
	}

	buf.WriteString("\r" + ClearLine + "\r")
}

// drawBoard renders the status board into the buffer
func (d *Display) drawBoard(buf *bytes.Buffer, state *tracking.Tracker) int {
	lineCount := 0
	separator := strings.Repeat("=", 70)

	// Header
	elapsed := d.tracker.GetElapsedTime().Round(time.Second)
	buf.WriteString(separator)
	buf.WriteString("\nFetching Activity Data | Elapsed: ")
	buf.WriteString(elapsed.String())
	buf.WriteByte('\n')
	buf.WriteString(separator)
	lineCount += 2

	for _, recType := range tracking.RecordingTypes {
		if traceState, exists := state.GetTrace(recType); exists {
			lineCount += d.writeRecordingTrace(buf, traceState)
		}
	}

	buf.WriteString(separator)

	return lineCount + 1
}

// writeRecordingTrace renders the trace for a specific recording type
func (d *Display) writeRecordingTrace(buf *bytes.Buffer, trace *tracking.TraceState) int {
	lines := 0

	icon := d.getStatusIcon(trace.Status)
	buf.WriteByte('\n')
	buf.WriteString(icon)
	buf.WriteString(" ")
	buf.WriteString(cases.Title(language.English).String(getEntityName(trace.Type, 0)))
	buf.WriteByte('\n')
	lines += 2

	switch trace.Status {
	case tracking.StatusCompleted:
		lines += d.writeLastPageInfo(buf, trace)
		lines += d.writeTotalFetchedInfo(buf, trace)
		lines += d.writeTotalFilteredInfo(buf, trace)
		lines += d.writeCompleteInfo(buf, trace)
	case tracking.StatusError:
		lines += d.writeSpanError(buf, trace)
	case tracking.StatusRunning:
		switch trace.CurrentSpan {
		case tracking.SpanFindLastPage:
			lines += d.writeFindLastPageInfo(buf, trace)
		case tracking.SpanFetchData:
			lines += d.writeLastPageInfo(buf, trace)
			lines += d.writeFetchInfo(buf, trace)
		case tracking.SpanFilter:
			lines += d.writeLastPageInfo(buf, trace)
			lines += d.writeTotalFetchedInfo(buf, trace)
			lines += d.writeTotalFilteredInfo(buf, trace)
		}
	}

	return lines
}

func (d *Display) writeFindLastPageInfo(buf *bytes.Buffer, trace *tracking.TraceState) int {
	lines := 0
	spinner := SpinnerFrames[d.spinnerFrame]

	buf.WriteString(spinner)
	buf.WriteString("   Probing last page...\n")
	lines++

	// Show binary search probe requests
	lines += d.writeRequests(buf, trace, d.config.MaxRequestsPerResource)

	return lines
}

func (d *Display) writeLastPageInfo(buf *bytes.Buffer, trace *tracking.TraceState) int {
	fmt.Fprintf(buf, "    Pages: %d\n", trace.Pagination.LastPageFound)
	return 1
}

func (d *Display) writeFetchInfo(buf *bytes.Buffer, trace *tracking.TraceState) int {
	lines := 0

	spinner := SpinnerFrames[d.spinnerFrame]
	buf.WriteString(spinner)
	buf.WriteString("   Fetching...\n")
	lines++

	lines += d.writeRequests(buf, trace, d.config.MaxRequestsPerResource)

	if len(trace.Network.RateLimitWaits) > 0 {
		lines += d.writeRateLimitWaits(buf, trace)
	}

	return lines
}

func (d *Display) writeTotalFetchedInfo(buf *bytes.Buffer, trace *tracking.TraceState) int {
	totalPages := trace.Pagination.LastPageFound
	fmt.Fprintf(buf, "    Fetched %d resources from %d pages\n", trace.Filter.TotalItems, totalPages)
	return 1
}

func (d *Display) writeTotalFilteredInfo(buf *bytes.Buffer, trace *tracking.TraceState) int {
	fmt.Fprintf(buf, "    Filtered: %d of %d %s\n", trace.Filter.TotalItemsAfterFilter, trace.Filter.TotalItems, getEntityName(trace.Type, trace.Filter.TotalItemsAfterFilter))
	return 1
}

func (d *Display) writeCompleteInfo(buf *bytes.Buffer, trace *tracking.TraceState) int {
	lines := 0

	// Use fixed completion time if available, otherwise calculate dynamically
	var elapsed time.Duration
	if !trace.CompletionTime.IsZero() {
		elapsed = trace.CompletionTime.Sub(trace.StartTime).Round(100 * time.Millisecond)
	} else {
		elapsed = time.Since(trace.StartTime).Round(100 * time.Millisecond)
	}

	cachePercent := 0
	totalCache := 0
	for _, req := range trace.Network.Requests {
		if req.Cached {
			totalCache++
		}
	}

	totalRequests := trace.Network.TotalRequests()
	if totalRequests > 0 {
		cachePercent = (totalCache * 100) / totalRequests
	}

	fmt.Fprintf(buf, "    ✓ Completed | %d cached of %d requests (%d%%) | %s\n",
		totalCache,
		totalRequests,
		cachePercent,
		elapsed.String(),
	)
	lines++

	return lines
}

func (d *Display) writeSpanError(buf *bytes.Buffer, trace *tracking.TraceState) int {
	buf.WriteString("    ERROR: ")
	if trace.Error != nil {
		buf.WriteString(trace.Error.Error())
	} else {
		buf.WriteString("Unknown error")
	}
	buf.WriteByte('\n')
	return 1
}

func (d *Display) writeRequests(buf *bytes.Buffer, trace *tracking.TraceState, maxDisplay int) int {
	lines := 0
	count := 0

	requests := make([]*tracking.RequestInfo, 0, len(trace.Network.Requests))
	for _, req := range trace.Network.Requests {
		requests = append(requests, req)
	}

	sort.Slice(requests, func(i, j int) bool {
		return requests[i].EndTime.After(requests[j].EndTime) && requests[i].StatusCode != 0
	})

	for _, req := range requests {
		if count >= maxDisplay {
			break
		}
		buf.WriteString("    └─ ")
		buf.WriteString(shortenURL(req.URL, 50))
		if req.StatusCode != 0 {
			fmt.Fprintf(buf, " [%d]", req.StatusCode)
		}
		if req.Cached {
			buf.WriteString(" (cached)")
		}
		buf.WriteByte('\n')
		lines++
		count++
	}

	// Show overflow indicator
	totalRequests := trace.Network.TotalRequests()
	if totalRequests > count {
		fmt.Fprintf(buf, "    └─ ... and %d more requests\n", totalRequests-count)
		lines++
	}

	return lines
}

func (d *Display) writeRateLimitWaits(buf *bytes.Buffer, trace *tracking.TraceState) int {
	if len(trace.Network.RateLimitWaits) > 0 {
		lastWait := trace.Network.RateLimitWaits[len(trace.Network.RateLimitWaits)-1]
		fmt.Fprintf(buf, "    └─ [Rate Limit] Waited %s (%s)\n",
			lastWait.WaitDuration*time.Second,
			lastWait.Reason,
		)
		return 1
	}
	return 0
}

func (d *Display) getStatusIcon(status tracking.OperationStatus) string {
	switch status {
	case tracking.StatusPending:
		return "[ ]"
	case tracking.StatusRunning:
		return "[~]"
	case tracking.StatusCompleted:
		return "[✓]"
	case tracking.StatusError:
		return "[✗]"
	default:
		return "[ ]"
	}
}

// getEntityName returns the entity name with proper pluralization
func getEntityName(recType tracking.ResourceType, count int) string {
	var name string

	switch recType {
	case tracking.Messages:
		name = "message"
	case tracking.Todos:
		name = "todo"
	case tracking.Todolists:
		name = "todolist"
	case tracking.Cards:
		name = "card"
	case tracking.Comments:
		name = "comment"
	case tracking.QuestionAnswers:
		name = "answer"
	default:
		name = "item"
	}

	if count <= 1 {
		return name
	}
	return name + "s"
}

func (d *Display) printFinalSummary() {
	state := d.tracker.GetState()
	if state == nil {
		return
	}

	elapsed := d.tracker.GetElapsedTime()
	separator := strings.Repeat("=", 70)

	fmt.Fprintf(d.writer, "\nActivity aggregation completed in %s\n%s\n", elapsed.Round(time.Second).String(), separator)
}

// shortenURL truncates URL to fit display
func shortenURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return "..." + url[len(url)-maxLen+3:]
}
