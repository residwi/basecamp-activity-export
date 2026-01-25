package export

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/residwi/basecamp-activity-export/internal/progress"
	"github.com/residwi/basecamp-activity-export/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExport(server *httptest.Server, tracker *trace.Tracer) *Export {
	client := basecamp.NewClient(
		basecamp.WithHTTPClient(server.Client()),
		basecamp.WithBaseURL(server.URL),
		basecamp.WithAccessToken("test-token"),
	)

	display := progress.NewDisplay(tracker)

	return New(
		WithBasecampClient(client),
		WithTracker(tracker),
		WithDisplay(display),
		WithFilename("test_report.md"),
	)
}

func TestNew(t *testing.T) {
	t.Run("creates export with default values", func(t *testing.T) {
		e := New()

		require.NotNil(t, e)
		assert.Nil(t, e.basecamp)
		assert.Nil(t, e.tracker)
		assert.Nil(t, e.display)
		assert.Empty(t, e.filename)
	})

	t.Run("applies all options", func(t *testing.T) {
		client := basecamp.NewClient()
		tracker := trace.NewTracer()
		display := progress.NewDisplay(tracker)

		e := New(
			WithBasecampClient(client),
			WithTracker(tracker),
			WithDisplay(display),
			WithFilename("report.md"),
		)

		assert.Equal(t, client, e.basecamp)
		assert.Equal(t, tracker, e.tracker)
		assert.Equal(t, display, e.display)
		assert.Equal(t, "report.md", e.filename)
	})

	t.Run("creates display when tracker provided but no display", func(t *testing.T) {
		tracker := trace.NewTracer()

		e := New(WithTracker(tracker))

		assert.NotNil(t, e.display)
	})

	t.Run("does not create display without tracker", func(t *testing.T) {
		e := New()

		assert.Nil(t, e.display)
	})
}

func TestExport_FetchActivity(t *testing.T) {
	t.Run("success fetches all activity types", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}
		now := time.Date(2025, time.February, 27, 10, 0, 0, 0, time.UTC)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Total-Count", "1")

			switch r.URL.Path {
			case "/my/profile.json":
				json.NewEncoder(w).Encode(profile)
			case "/projects/recordings.json":
				recordingType := r.URL.Query().Get("type")
				switch recordingType {
				case "Message":
					json.NewEncoder(w).Encode([]basecamp.Message{{
						Recording: basecamp.Recording{
							ID:        1,
							Title:     "Test Message",
							Creator:   profile,
							CreatedAt: now,
						},
					}})
				case "Todo":
					json.NewEncoder(w).Encode([]basecamp.Todo{{
						Recording: basecamp.Recording{
							ID:        2,
							Title:     "Test Todo",
							Creator:   profile,
							CreatedAt: now,
						},
					}})
				case "Todolist":
					json.NewEncoder(w).Encode([]basecamp.Todolist{{
						Recording: basecamp.Recording{
							ID:        4,
							Title:     "Test Todolist",
							Creator:   profile,
							CreatedAt: now,
						},
					}})
				case "Kanban::Card":
					json.NewEncoder(w).Encode([]basecamp.KanbanCard{{
						Recording: basecamp.Recording{
							ID:        5,
							Title:     "Test KanbanCard",
							Creator:   profile,
							CreatedAt: now,
						},
					}})
				case "Comment":
					json.NewEncoder(w).Encode([]basecamp.Comment{{
						ID:        6,
						Content:   "Test Comment",
						Creator:   profile,
						CreatedAt: now,
					}})
				case "Question::Answer":
					json.NewEncoder(w).Encode([]basecamp.QuestionAnswer{{
						Recording: basecamp.Recording{
							ID:        7,
							Title:     "Test QuestionAnswer",
							Creator:   profile,
							CreatedAt: now,
						},
					}})
				default:
					json.NewEncoder(w).Encode([]any{})
				}
			default:
				http.NotFound(w, r)
			}
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, 2025, result.Year())
		assert.Equal(t, profile.Name, result.User().Name)
		assert.Len(t, result.MessagesCreated(), 1)
		assert.Len(t, result.TodosCreated(), 1)
		assert.Len(t, result.TodolistsCreated(), 1)
		assert.Len(t, result.KanbanCardsCreated(), 1)
		assert.Len(t, result.CommentsWritten(), 1)
		assert.Len(t, result.QuestionAnswers(), 1)
	})

	t.Run("returns error when profile fetch fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/my/profile.json" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}))
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "unauthorized")
	})

	t.Run("returns error when messages fetch fails", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			if r.URL.Query().Get("type") == "Message" {
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "server error")
	})

	t.Run("returns error when todos fetch fails", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			if r.URL.Query().Get("type") == "Todo" {
				http.Error(w, "todo server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "todo server error")
	})

	t.Run("returns error when todolists fetch fails", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			if r.URL.Query().Get("type") == "Todolist" {
				http.Error(w, "todolists server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "todolists server error")
	})

	t.Run("returns error when kanban cards fetch fails", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			if r.URL.Query().Get("type") == "Kanban::Card" {
				http.Error(w, "KanbanCard server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "KanbanCard server error")
	})

	t.Run("returns error when comments fetch fails", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			if r.URL.Query().Get("type") == "Comment" {
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "server error")
	})

	t.Run("returns error when question answers fetch fails", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			if r.URL.Query().Get("type") == "Question::Answer" {
				http.Error(w, "questionAnswer server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		result, err := e.FetchActivity(context.Background(), 2025)
		require.Error(t, err)

		assert.Nil(t, result)
		assert.ErrorContains(t, err, "questionAnswer server error")
	})

	t.Run("context cancellation stops fetching", func(t *testing.T) {
		profile := basecamp.Person{ID: 123, Name: "Test User"}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/my/profile.json" {
				json.NewEncoder(w).Encode(profile)
				return
			}

			// simulate slow response
			time.Sleep(100 * time.Millisecond)
			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]any{})
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		tracker := trace.NewTracer()
		e := newTestExport(server, tracker)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := e.FetchActivity(ctx, 2025)
		require.Error(t, err)

		assert.ErrorIs(t, err, context.Canceled)
	})
}
