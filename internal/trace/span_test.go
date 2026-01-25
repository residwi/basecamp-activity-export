package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpan(t *testing.T) {
	t.Run("constants have expected values", func(t *testing.T) {
		tests := []struct {
			span     Span
			expected string
		}{
			{SpanFindLastPage, "find_last_page"},
			{SpanFetchData, "fetch_data"},
			{SpanFilter, "filter"},
		}

		for _, tt := range tests {
			assert.Equal(t, Span(tt.expected), tt.span)
		}
	})
}

func TestStatus(t *testing.T) {
	t.Run("iota values are sequential from zero", func(t *testing.T) {
		tests := []struct {
			status   Status
			expected int
		}{
			{StatusPending, 0},
			{StatusRunning, 1},
			{StatusCompleted, 2},
			{StatusError, 3},
		}

		for _, tt := range tests {
			assert.Equal(t, Status(tt.expected), tt.status)
		}
	})
}
