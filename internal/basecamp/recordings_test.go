package basecamp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RecordingTypes(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) (any, error)
	}{
		{
			name: "GetTodos",
			call: func(c *Client) (any, error) {
				return c.GetTodos(t.Context(), 2025)
			},
		},
		{
			name: "GetMessages",
			call: func(c *Client) (any, error) {
				return c.GetMessages(t.Context(), 2025)
			},
		},
		{
			name: "GetTodolists",
			call: func(c *Client) (any, error) {
				return c.GetTodolists(t.Context(), 2025)
			},
		},
		{
			name: "GetKanbanCards",
			call: func(c *Client) (any, error) {
				return c.GetKanbanCards(t.Context(), 2025)
			},
		},
		{
			name: "GetComments",
			call: func(c *Client) (any, error) {
				return c.GetComments(t.Context(), 2025)
			},
		},
		{
			name: "GetQuestionAnswers",
			call: func(c *Client) (any, error) {
				return c.GetQuestionAnswers(t.Context(), 2025)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/projects/recordings.json", r.URL.Path)
				_ = json.NewEncoder(w).Encode([]json.RawMessage{
					json.RawMessage(`{"id":1}`),
				})
			}))
			defer server.Close()

			client := newTestClient(server)

			res, err := tt.call(client)
			require.NoError(t, err)

			assert.NotNil(t, res)
		})

		t.Run(tt.name+" on error", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "server error", http.StatusInternalServerError)
			}))
			defer server.Close()

			client := newTestClient(server)

			res, err := tt.call(client)
			require.Error(t, err)

			assert.ErrorContains(t, err, "server error")
			assert.Nil(t, res)
		})
	}
}

func TestGetRecordingsTyped(t *testing.T) {
	t.Run("success with recording type filter", func(t *testing.T) {
		recordings := []Recording{
			{ID: 1, Title: "Todo 1", CreatedAt: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)},
			{ID: 2, Title: "Todo 2", CreatedAt: time.Date(2025, 7, 20, 9, 30, 0, 0, time.UTC)},
			{ID: 3, Title: "Todo 3", CreatedAt: time.Date(2025, 8, 5, 14, 45, 0, 0, time.UTC)},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/projects/recordings.json", r.URL.Path)
			assert.Equal(t, "Todo", r.URL.Query().Get("type"))

			w.Header().Set("X-Total-Count", "3")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(recordings)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := GetRecordingsTyped[Recording](client, t.Context(), "Todo", 2025)
		require.NoError(t, err)

		assert.Len(t, results, 3)
		assertResultsEqual(t, recordings, results)
	})

	t.Run("success without recording type filter", func(t *testing.T) {
		recordings := []Recording{
			{ID: 1, Title: "General Recording", CreatedAt: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.URL.Query().Get("type"))

			w.Header().Set("X-Total-Count", "1")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(recordings)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := GetRecordingsTyped[Recording](client, t.Context(), "", 2025)
		require.NoError(t, err)

		assert.Len(t, results, 1)
		assertResultsEqual(t, recordings, results)
	})

	t.Run("error from GetPaginated", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := GetRecordingsTyped[Recording](client, t.Context(), "Todo", 2025)
		require.Error(t, err)

		assert.ErrorContains(t, err, "server error")
		assert.Nil(t, results)
	})

	t.Run("skips malformed JSON entries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "3")
			w.Header().Set("Content-Type", "application/json")

			items := []json.RawMessage{
				json.RawMessage(`{"id": 1, "title": "Valid 1", "created_at": "2025-06-15T12:00:00Z"}`),
				json.RawMessage(`[]`), // Malformed entry (should be an object)
				json.RawMessage(`{"id": 3, "title": "Valid 2", "created_at": "2025-06-15T12:00:00Z"}`),
			}
			json.NewEncoder(w).Encode(items)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := GetRecordingsTyped[Recording](client, t.Context(), "", 2025)
		require.NoError(t, err)

		assert.Len(t, results, 2)
		assert.Equal(t, int64(1), results[0].ID)
		assert.Equal(t, "Valid 1", results[0].Title)
		assert.Equal(t, int64(3), results[1].ID)
		assert.Equal(t, "Valid 2", results[1].Title)
	})

	t.Run("empty results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "0")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Recording{})
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := GetRecordingsTyped[Recording](client, t.Context(), "Todo", 2025)
		require.NoError(t, err)

		assert.Empty(t, results)
	})
}

func TestTodo_IsCompletedByUser(t *testing.T) {
	tests := []struct {
		name     string
		todo     Todo
		userID   int64
		expected bool
	}{
		{
			name:     "nil completion",
			todo:     Todo{Completion: nil},
			userID:   123,
			expected: false,
		},
		{
			name:     "completed and matching user",
			todo:     Todo{Completed: true, Completion: &Completion{Creator: Person{ID: 123}}},
			userID:   123,
			expected: true,
		},
		{
			name:     "completed but non-matching user",
			todo:     Todo{Completed: true, Completion: &Completion{Creator: Person{ID: 123}}},
			userID:   456,
			expected: false,
		},
		{
			name:     "not completed",
			todo:     Todo{Completed: false, Completion: &Completion{Creator: Person{ID: 123}}},
			userID:   123,
			expected: false,
		},
		{
			name:     "completed with nil creator",
			todo:     Todo{Completed: true, Completion: &Completion{Creator: Person{}}},
			userID:   123,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.todo.IsCompletedByUser(tt.userID))
		})
	}
}

func TestTodo_IsCompletedInYear(t *testing.T) {
	completedAt := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		todo     Todo
		year     int
		expected bool
	}{
		{
			name:     "nil completion",
			todo:     Todo{Completion: nil},
			year:     2025,
			expected: false,
		},
		{
			name:     "completed in matching year",
			todo:     Todo{Completed: true, Completion: &Completion{CreatedAt: &completedAt}},
			year:     2025,
			expected: true,
		},
		{
			name:     "completed in non-matching year",
			todo:     Todo{Completed: true, Completion: &Completion{CreatedAt: &completedAt}},
			year:     2024,
			expected: false,
		},
		{
			name:     "not completed",
			todo:     Todo{Completed: false, Completion: &Completion{CreatedAt: &completedAt}},
			year:     2025,
			expected: false,
		},
		{
			name:     "completed with nil createdAt",
			todo:     Todo{Completed: true, Completion: &Completion{CreatedAt: nil}},
			year:     2025,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.todo.IsCompletedInYear(tt.year))
		})
	}
}

func TestIsAssigned(t *testing.T) {
	tests := []struct {
		name      string
		assignees []Person
		userID    int64
		expected  bool
	}{
		{
			name: "assigned to user",
			assignees: []Person{
				{ID: 123, Name: "User 1"},
				{ID: 456, Name: "User 2"},
			},
			userID:   123,
			expected: true,
		},
		{
			name: "not assigned to user",
			assignees: []Person{
				{ID: 123, Name: "User 1"},
			},
			userID:   456,
			expected: false,
		},
		{
			name:      "no assignees",
			assignees: []Person{},
			userID:    123,
			expected:  false,
		},
		{
			name:      "nil assignees",
			assignees: nil,
			userID:    123,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAssigned(tt.assignees, tt.userID))
		})
	}
}

func TestIsCreatedByUser(t *testing.T) {
	tests := []struct {
		name     string
		creator  Person
		userID   int64
		expected bool
	}{
		{
			name:     "matching user",
			creator:  Person{ID: 123},
			userID:   123,
			expected: true,
		},
		{
			name:     "non-matching user",
			creator:  Person{ID: 123},
			userID:   456,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsCreatedByUser(tt.creator, tt.userID))
		})
	}
}

func TestIsInYear(t *testing.T) {
	tests := []struct {
		name      string
		createdAt time.Time
		year      int
		expected  bool
	}{
		{
			name:      "matching year",
			createdAt: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			year:      2025,
			expected:  true,
		},
		{
			name:      "non-matching year",
			createdAt: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			year:      2024,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsInYear(tt.createdAt, tt.year))
		})
	}
}
