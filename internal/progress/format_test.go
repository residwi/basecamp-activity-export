package progress

import (
	"testing"

	"github.com/residwi/basecamp-activity-export/internal/trace"
	"github.com/stretchr/testify/assert"
)

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		status   trace.Status
		expected string
	}{
		{
			name:     "pending status",
			status:   trace.StatusPending,
			expected: "[ ]",
		},
		{
			name:     "running status",
			status:   trace.StatusRunning,
			expected: "[~]",
		},
		{
			name:     "completed status",
			status:   trace.StatusCompleted,
			expected: "[✓]",
		},
		{
			name:     "error status",
			status:   trace.StatusError,
			expected: "[✗]",
		},
		{
			name:     "unknown status defaults to pending icon",
			status:   trace.Status(999),
			expected: "[ ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := statusIcon(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEntityName(t *testing.T) {
	t.Run("resource types", func(t *testing.T) {
		tests := []struct {
			name         string
			resourceType trace.ResourceType
			count        int
			expected     string
		}{
			// Singular forms (count == 1)
			{"messages singular", trace.Messages, 1, "message"},
			{"todos singular", trace.Todos, 1, "todo"},
			{"todolists singular", trace.Todolists, 1, "todolist"},
			{"kanban cards singular", trace.KanbanCards, 1, "kanban card"},
			{"comments singular", trace.Comments, 1, "comment"},
			{"question answers singular", trace.QuestionAnswers, 1, "answer"},

			// Plural forms (count == 0)
			{"messages plural zero", trace.Messages, 0, "messages"},
			{"todos plural zero", trace.Todos, 0, "todos"},
			{"todolists plural zero", trace.Todolists, 0, "todolists"},
			{"kanban cards plural zero", trace.KanbanCards, 0, "kanban cards"},
			{"comments plural zero", trace.Comments, 0, "comments"},
			{"question answers plural zero", trace.QuestionAnswers, 0, "answers"},

			// Plural forms (count > 1)
			{"messages plural multiple", trace.Messages, 5, "messages"},
			{"todos plural multiple", trace.Todos, 10, "todos"},

			// Unknown type
			{"unknown type singular", "unknown-resource-type", 1, "item"},
			{"unknown type plural", "unknown-resource-type", 2, "items"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := entityName(tt.resourceType, tt.count)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestShortenURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		maxLen   int
		expected string
	}{
		{
			name:     "url shorter than max length",
			url:      "https://example.com",
			maxLen:   50,
			expected: "https://example.com",
		},
		{
			name:     "url equal to max length",
			url:      "https://example.com/path",
			maxLen:   24,
			expected: "https://example.com/path",
		},
		{
			name:     "url longer than max length",
			url:      "https://example.com/very/long/path/to/resource",
			maxLen:   20,
			expected: ".../path/to/resource",
		},
		{
			name:     "very short max length",
			url:      "https://example.com/path",
			maxLen:   10,
			expected: "...om/path",
		},
		{
			name:     "empty url",
			url:      "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenURL(tt.url, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			if tt.url != "" && len(tt.url) > tt.maxLen {
				assert.LessOrEqual(t, len(result), tt.maxLen)
			}
		})
	}
}
