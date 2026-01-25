package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testResourceType trace.ResourceType = "stub-resource"

func TestNewClient(t *testing.T) {
	t.Run("with options", func(t *testing.T) {
		httpClient := &http.Client{Timeout: 30 * time.Second}

		client := NewClient(
			WithHTTPClient(httpClient),
			WithAccountID("12345"),
			WithAccessToken("secret-token"),
			WithBaseURL("https://api.example.com"),
			WithUserAgent("TestAgent/1.0"),
		)

		assert.Equal(t, httpClient, client.httpClient)
		assert.Equal(t, "12345", client.accountID)
		assert.Equal(t, "secret-token", client.accessToken)
		assert.Equal(t, "https://api.example.com", client.baseURL)
		assert.Equal(t, "TestAgent/1.0", client.userAgent)
	})

	t.Run("with concurrency limit", func(t *testing.T) {
		client := NewClient(WithConcurrencyLimit(40))

		assert.NotNil(t, client.semaphore)
		assert.Equal(t, 40, cap(client.semaphore), "should use specified concurrency limit")
	})
}

func TestClient_get(t *testing.T) {
	t.Run("success with result", func(t *testing.T) {
		expected := testItem{ID: 1, Title: "Test", CreatedAt: time.Now().UTC().Truncate(time.Second)}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(expected)
		}))
		defer server.Close()

		client := newTestClient(server)

		var result testItem
		err := client.Get(t.Context(), "/items/1", nil, &result)
		require.NoError(t, err)

		assert.Equal(t, expected.ID, result.ID)
		assert.Equal(t, expected.Title, result.Title)
	})

	t.Run("success with query parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))
			assert.Equal(t, "value2", r.URL.Query().Get("param2"))
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer server.Close()

		client := newTestClient(server)
		query := url.Values{"param1": {"value1"}, "param2": {"value2"}}

		var result map[string]string
		err := client.Get(t.Context(), "/test", query, &result)
		require.NoError(t, err)

		assert.Equal(t, "ok", result["status"])
	})

	t.Run("nil result skips decoding", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		client := newTestClient(server)

		err := client.Get(t.Context(), "/test", nil, nil)
		require.NoError(t, err)
	})

	t.Run("execute error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)

		var result testItem
		err := client.Get(t.Context(), "/test", nil, &result)
		require.Error(t, err)

		assert.ErrorContains(t, err, "API error 500")
	})

	t.Run("decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json{{{"))
		}))
		defer server.Close()

		client := newTestClient(server)

		var result testItem
		err := client.Get(t.Context(), "/test", nil, &result)
		require.Error(t, err)

		assert.ErrorContains(t, err, "decoding response")
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			json.NewEncoder(w).Encode(testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		var result testItem
		err := client.Get(ctx, "/test", nil, &result)
		require.Error(t, err)

		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestClient_getPaginated(t *testing.T) {
	t.Run("empty first page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "0")
			json.NewEncoder(w).Encode([]testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := client.GetPaginated(t.Context(), "/items", nil, 2025)
		require.NoError(t, err)

		assert.Empty(t, results)
	})

	t.Run("year boundary in first page", func(t *testing.T) {
		items := []testItem{
			makeTestItem(1, 2025),
			makeTestItem(2, 2025),
			makeTestItem(3, 2024),
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "3")
			json.NewEncoder(w).Encode(items)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := client.GetPaginated(t.Context(), "/items", nil, 2025)
		require.NoError(t, err)

		assert.Len(t, results, 2)
		assertResultsEqual(t, items[:2], results)
	})

	t.Run("single page total maxPages equals 1", func(t *testing.T) {
		items := makeTestItemsSameYear(1, 10, 2025)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "10")
			json.NewEncoder(w).Encode(items)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := client.GetPaginated(t.Context(), "/items", nil, 2025)
		require.NoError(t, err)

		assert.Len(t, results, 10)
		assertResultsEqual(t, items, results)
	})

	t.Run("multiple pages full flow success", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.totalItems = 100

		handler.setPage(1, makeTestItemsSameYear(1, 15, 2025))
		handler.setPage(2, makeTestItemsSameYear(16, 30, 2025))
		handler.setPage(3, makeTestItemsSameYear(46, 50, 2025))
		handler.setPage(4, makeTestItemsSameYear(96, 5, 2025))

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		results, err := client.GetPaginated(t.Context(), "/items", nil, 2025)
		require.NoError(t, err)

		assert.Len(t, results, 100)
	})

	t.Run("fetch first page error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		results, err := client.GetPaginated(t.Context(), "/items", nil, 2025)
		require.Error(t, err)

		assert.Nil(t, results)
	})

	t.Run("findLastPage error propagates and fails trace", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount == 1 {
				w.Header().Set("X-Total-Count", "100")
				json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 15, 2025))
				return
			}
			http.Error(w, "probe error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()

		results, err := client.GetPaginated(ctx, "/items", nil, 2025)

		require.Error(t, err)
		assert.Nil(t, results)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)

		assert.Equal(t, trace.StatusError, state.Status)
		assert.Contains(t, state.Error.Error(), "probe error")
	})

	t.Run("with tracking context sets metadata", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.totalItems = 100

		handler.setPage(1, makeTestItemsSameYear(1, 15, 2025))
		handler.setPage(2, makeTestItemsSameYear(16, 30, 2025))
		handler.setPage(3, makeTestItemsSameYear(46, 50, 2025))
		handler.setPage(4, makeTestItemsSameYear(96, 5, 2025))

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()

		results, err := client.GetPaginated(ctx, "/items", nil, 2025)
		require.NoError(t, err)

		assert.Len(t, results, 100)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)

		assert.Equal(t, 100, state.Filter.TotalItems)
		assert.Equal(t, 4, state.Pagination.LastPageFound)
	})

	t.Run("fetchPagesParallel error propagates and fails trace", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.totalItems = 195

		handler.setPage(1, makeTestItemsSameYear(1, 15, 2025))
		handler.setPage(2, makeTestItemsSameYear(16, 30, 2025))
		handler.setPage(3, makeTestItemsSameYear(46, 50, 2025))
		handler.setPage(4, makeTestItemsSameYear(96, 100, 2025))
		handler.failOnPage = 3

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()

		results, err := client.GetPaginated(ctx, "/items", nil, 2025)
		require.Error(t, err)

		assert.Nil(t, results)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)

		assert.Equal(t, trace.StatusError, state.Status)
		assert.Contains(t, state.Error.Error(), "simulated fetch error on page 3")
	})
}

func TestClient_fetchFirstPage(t *testing.T) {
	t.Run("success with total count header", func(t *testing.T) {
		items := makeTestItems(1, 5, 2025)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			w.Header().Set("X-Total-Count", "999")
			json.NewEncoder(w).Encode(items)
		}))
		defer server.Close()

		client := newTestClient(server)
		page, total, err := client.fetchFirstPage(t.Context(), "/items", nil)
		require.NoError(t, err)

		assert.Len(t, page, 5)
		assert.Equal(t, 999, total)
	})

	t.Run("with existing query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			assert.Equal(t, "extra", r.URL.Query().Get("filter"))
			w.Header().Set("X-Total-Count", "10")
			json.NewEncoder(w).Encode([]testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		query := url.Values{"filter": {"extra"}}
		_, total, err := client.fetchFirstPage(t.Context(), "/items", query)
		require.NoError(t, err)

		assert.Equal(t, 10, total)
	})

	t.Run("execute error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "error", http.StatusBadGateway)
		}))
		defer server.Close()

		client := newTestClient(server)
		_, _, err := client.fetchFirstPage(t.Context(), "/items", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "API error 502")
	})

	t.Run("decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "10")
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		client := newTestClient(server)
		_, _, err := client.fetchFirstPage(t.Context(), "/items", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "decoding first page")
	})

	t.Run("missing total count header defaults to zero", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		_, total, err := client.fetchFirstPage(t.Context(), "/items", nil)
		require.NoError(t, err)

		assert.Equal(t, 0, total)
	})

	t.Run("with tracking context records total items", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "42")
			json.NewEncoder(w).Encode([]testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()
		items, totalItems, err := client.fetchFirstPage(ctx, "/items", nil)
		require.NoError(t, err)

		assert.Empty(t, items)
		assert.Equal(t, 42, totalItems)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)

		assert.Equal(t, 42, state.Filter.TotalItems)
	})
}

func TestClient_findLastPage(t *testing.T) {
	t.Run("probePage error on second page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "stub-error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		lastPage, err := client.findLastPageByYear(t.Context(), "/items", nil, 2025, 10)
		require.Error(t, err)

		assert.Equal(t, 1, lastPage) // since probePage failed on page 2, lastGood is 1
		assert.ErrorContains(t, err, "stub-error")
	})

	t.Run("no data on second page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		lastPage, err := client.findLastPageByYear(t.Context(), "/items", nil, 2025, 10)
		require.NoError(t, err)

		assert.Equal(t, 1, lastPage)
	})

	t.Run("no target year on second page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2022)})
		}))
		defer server.Close()

		client := newTestClient(server)
		lastPage, err := client.findLastPageByYear(t.Context(), "/items", nil, 2025, 10)
		require.NoError(t, err)

		assert.Equal(t, 1, lastPage)
	})

	t.Run("finds boundary through gallop and binary search", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		for i := 1; i <= 5; i++ {
			handler.setPage(i, makeTestItemsSameYear(i*10, 10, 2025))
		}
		for i := 6; i <= 10; i++ {
			handler.setPage(i, makeTestItemsSameYear(i*10, 10, 2024))
		}
		handler.totalItems = 500

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		lastPage, err := client.findLastPageByYear(t.Context(), "/items", nil, 2025, 10)
		require.NoError(t, err)

		assert.Equal(t, 5, lastPage)
	})

	t.Run("gallopSearch error propagates with partial result", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))

			if page <= 2 {
				json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
				return
			}
			http.Error(w, "stub-error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		lastPage, err := client.findLastPageByYear(t.Context(), "/items", nil, 2025, 10)
		require.Error(t, err)

		assert.Equal(t, 2, lastPage) // lastGood from gallopSearch
		assert.ErrorContains(t, err, "stub-error")
	})

	t.Run("with tracking context records spans", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()

		lastPage, err := client.findLastPageByYear(ctx, "/items", nil, 2025, 1)
		require.NoError(t, err)

		assert.Equal(t, 1, lastPage)

		_, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)
	})
}

func TestClient_gallopSearch(t *testing.T) {
	t.Run("context cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, _, err := client.gallopSearch(ctx, "/items", nil, 2025, 10)
		require.Error(t, err)

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("probePage error returns partial result", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount > 2 {
				http.Error(w, "stub-error", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
		}))
		defer server.Close()

		client := newTestClient(server)
		lastGood, upperBound, err := client.gallopSearch(t.Context(), "/items", nil, 2025, 100)
		require.Error(t, err)

		assert.Equal(t, 4, lastGood)   // last successful probe was page 4
		assert.Equal(t, 8, upperBound) // last probe was page 8
	})

	t.Run("reaches maxPages exactly and returns early", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
		}))
		defer server.Close()

		client := newTestClient(server)
		lastGood, upperBound, err := client.gallopSearch(t.Context(), "/items", nil, 2025, 4)
		require.NoError(t, err)

		assert.Equal(t, 4, lastGood)
		assert.Equal(t, 4, upperBound)
	})

	t.Run("finds bad page early returns bounds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page > 2 {
				json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2020))
			} else {
				json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
			}
		}))
		defer server.Close()

		client := newTestClient(server)
		lastGood, upperBound, err := client.gallopSearch(t.Context(), "/items", nil, 2025, 100)

		require.NoError(t, err)
		assert.Equal(t, 2, lastGood)
		assert.Equal(t, 4, upperBound)
	})

	t.Run("upperBound equals maxPages exits loop", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 10, 2025))
		}))
		defer server.Close()

		client := newTestClient(server)
		lastGood, upperBound, err := client.gallopSearch(t.Context(), "/items", nil, 2025, 1)

		require.NoError(t, err)
		assert.Equal(t, 1, lastGood)
		assert.Equal(t, 1, upperBound)
	})
}

func TestClient_binarySearch(t *testing.T) {
	t.Run("context cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2025)})
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		result, err := client.binarySearch(ctx, "/items", nil, 2025, 1, 10)
		require.Error(t, err)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, result)
	})

	t.Run("probePage error returns low with error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "stub-error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		result, err := client.binarySearch(t.Context(), "/items", nil, 2025, 1, 10)
		require.Error(t, err)

		assert.ErrorContains(t, err, "stub-error")
		assert.Equal(t, 1, result)
	})

	t.Run("narrows down to exact page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page <= 5 {
				json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2025)})
			} else {
				json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2020)})
			}
		}))
		defer server.Close()

		client := newTestClient(server)
		result, err := client.binarySearch(t.Context(), "/items", nil, 2025, 1, 10)
		require.NoError(t, err)

		assert.Equal(t, 5, result)
	})

	t.Run("low equals high-1 exits immediately", func(t *testing.T) {
		var requestCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2025)})
		}))
		defer server.Close()

		client := newTestClient(server)
		result, err := client.binarySearch(t.Context(), "/items", nil, 2025, 5, 6)
		require.NoError(t, err)

		assert.Equal(t, 5, result)
		assert.Equal(t, 0, requestCount) // no probes made
	})
}

func TestClient_fetchPagesParallel(t *testing.T) {
	t.Run("lastPage equals 1 returns filtered first page", func(t *testing.T) {
		client := NewClient(WithConcurrencyLimit(20))
		firstPage := itemsToRawMessages([]testItem{
			makeTestItem(1, 2025),
			makeTestItem(2, 2024),
		})

		results, err := client.fetchPagesParallel(t.Context(), "/items", nil, 2025, 1, firstPage)
		require.NoError(t, err)

		assertResultsEqual(t, []testItem{makeTestItem(1, 2025)}, results)
		assert.Len(t, results, 1)
	})

	t.Run("fetches multiple pages in parallel", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.setPage(1, makeTestItems(1, 10, 2025))
		handler.setPage(2, makeTestItems(11, 10, 2025))
		handler.setPage(3, makeTestItems(21, 10, 2025))
		handler.totalItems = 30

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		firstPage := itemsToRawMessages(handler.pages[1])

		results, err := client.fetchPagesParallel(t.Context(), "/items", nil, 2025, 3, firstPage)
		require.NoError(t, err)

		assert.Len(t, results, 30)
		assertResultsEqual(t, append(append(handler.pages[1], handler.pages[2]...), handler.pages[3]...), results)
	})

	t.Run("handles worker errors", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.setPage(1, makeTestItems(1, 10, 2025))
		handler.failOnPage = 2
		handler.totalItems = 30

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		firstPage := itemsToRawMessages(handler.pages[1])

		results, err := client.fetchPagesParallel(t.Context(), "/items", nil, 2025, 3, firstPage)
		require.Error(t, err)

		assert.Nil(t, results)
	})

	t.Run("context cancellation during fetch", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.setPage(1, makeTestItems(1, 10, 2025))
		handler.setPage(2, makeTestItems(11, 10, 2025))
		handler.delayOnPage = 2
		handler.delay = 20 * time.Millisecond
		handler.totalItems = 20

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		firstPage := itemsToRawMessages(handler.pages[1])

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()

		results, err := client.fetchPagesParallel(ctx, "/items", nil, 2025, 2, firstPage)
		require.Error(t, err)

		assert.Nil(t, results)
	})

	t.Run("with tracking context records span", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.setPage(1, makeTestItems(1, 10, 2025))
		handler.setPage(2, makeTestItems(11, 10, 2025))
		handler.totalItems = 20

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()
		firstPage := itemsToRawMessages(handler.pages[1])

		results, err := client.fetchPagesParallel(ctx, "/items", nil, 2025, 2, firstPage)
		require.NoError(t, err)

		assert.Len(t, results, 20)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)

		assert.Equal(t, trace.SpanFetchData, state.CurrentSpan)
		assertResultsEqual(t, append(handler.pages[1], handler.pages[2]...), results)
	})

	t.Run("semaphore limits concurrency", func(t *testing.T) {
		var activeCount atomic.Int32
		var maxActive atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := activeCount.Add(1)
			defer activeCount.Add(-1)

			time.Sleep(10 * time.Millisecond)

			for {
				old := maxActive.Load()
				if current <= old || maxActive.CompareAndSwap(old, current) {
					break
				}
			}

			pageQuery, _ := strconv.Atoi(r.URL.Query().Get("page"))
			json.NewEncoder(w).Encode([]testItem{makeTestItem(pageQuery, 2025)})
		}))
		defer server.Close()

		client := newTestClientWithSemaphore(server, 3)
		firstPage := itemsToRawMessages([]testItem{makeTestItem(1, 2025)})

		results, err := client.fetchPagesParallel(t.Context(), "/items", nil, 2025, 10, firstPage)
		require.NoError(t, err)

		assert.Equal(t, int32(3), maxActive.Load())
		assert.Len(t, results, 10)
		assertResultsEqual(t, makeTestItemsSameYear(1, 10, 2025), results)
	})

	t.Run("all workers complete before channel close", func(t *testing.T) {
		completedWorkers := atomic.Int32{}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			completedWorkers.Add(1)

			pageQuery, _ := strconv.Atoi(r.URL.Query().Get("page"))
			json.NewEncoder(w).Encode([]testItem{makeTestItem(pageQuery, 2025)})
		}))
		defer server.Close()

		client := newTestClient(server)
		firstPage := itemsToRawMessages([]testItem{makeTestItem(1, 2025)})

		results, err := client.fetchPagesParallel(t.Context(), "/items", nil, 2025, 5, firstPage)
		require.NoError(t, err)

		assert.Equal(t, int32(4), completedWorkers.Load())
		assert.Len(t, results, 5)
		assertResultsEqual(t, makeTestItemsSameYear(1, 5, 2025), results)
	})
}

func TestClient_fetchPageWorker(t *testing.T) {
	t.Run("success flow", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2025)})
			}))
			defer server.Close()

			client := newTestClient(server)
			results := make(chan pageResult, 1)
			var wg sync.WaitGroup
			wg.Add(1)

			go client.fetchPageWorker(t.Context(), &wg, results, "/items", nil, 2)

			wg.Wait()
			close(results)

			res := <-results
			require.NoError(t, res.err)

			assert.Equal(t, 2, res.pageNum)
			assert.Len(t, res.items, 1)
		})
	})

	t.Run("context cancelled while waiting for semaphore", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode([]testItem{})
			}))
			defer server.Close()

			client := newTestClientWithSemaphore(server, 1)
			client.semaphore <- struct{}{}

			ctx, cancel := context.WithCancel(t.Context())
			results := make(chan pageResult, 1)
			var wg sync.WaitGroup
			wg.Add(1)

			go client.fetchPageWorker(ctx, &wg, results, "/items", nil, 2)

			cancel()
			wg.Wait()
			close(results)

			<-client.semaphore

			res := <-results
			require.Error(t, res.err)

			assert.ErrorIs(t, res.err, context.Canceled)
		})
	})

	t.Run("fetch error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "stub-error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		results := make(chan pageResult, 1)
		var wg sync.WaitGroup
		wg.Add(1)

		go client.fetchPageWorker(t.Context(), &wg, results, "/items", nil, 2)

		wg.Wait()
		close(results)

		res := <-results
		require.Error(t, res.err)

		assert.ErrorContains(t, res.err, "stub-error")
	})
}

func TestClient_probePage(t *testing.T) {
	t.Run("success with target year", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "5", r.URL.Query().Get("page"))
			json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2025)})
		}))
		defer server.Close()

		client := newTestClient(server)
		hasData, hasYear, err := client.probePage(t.Context(), "/items", nil, 2025, 5)
		require.NoError(t, err)

		assert.True(t, hasData)
		assert.True(t, hasYear)
	})

	t.Run("success without target year", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]testItem{makeTestItem(1, 2022)})
		}))
		defer server.Close()

		client := newTestClient(server)
		hasData, hasYear, err := client.probePage(t.Context(), "/items", nil, 2025, 1)
		require.NoError(t, err)

		assert.True(t, hasData)
		assert.False(t, hasYear)
	})

	t.Run("empty page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]testItem{})
		}))
		defer server.Close()

		client := newTestClient(server)
		hasData, hasYear, err := client.probePage(t.Context(), "/items", nil, 2025, 1)
		require.NoError(t, err)

		assert.False(t, hasData)
		assert.False(t, hasYear)
	})

	t.Run("context cancelled while waiting for semaphore", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode([]testItem{})
			}))
			defer server.Close()

			client := newTestClientWithSemaphore(server, 1)
			client.semaphore <- struct{}{}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			hasData, hasYear, err := client.probePage(ctx, "/items", nil, 2025, 1)
			require.Error(t, err)

			<-client.semaphore

			assert.ErrorIs(t, err, context.Canceled)
			assert.False(t, hasData)
			assert.False(t, hasYear)
		})
	})

	t.Run("execute error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "stub-error", http.StatusBadRequest)
		}))
		defer server.Close()

		client := newTestClient(server)
		hasData, hasYear, err := client.probePage(t.Context(), "/items", nil, 2025, 1)
		require.Error(t, err)

		assert.ErrorContains(t, err, "stub-error")
		assert.False(t, hasData)
		assert.False(t, hasYear)
	})

	t.Run("decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		client := newTestClient(server)
		hasData, hasYear, err := client.probePage(t.Context(), "/items", nil, 2025, 1)
		require.Error(t, err)

		assert.ErrorContains(t, err, "decoding probe page 1")
		assert.False(t, hasData)
		assert.False(t, hasYear)
	})
}

func TestClient_execute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(server)
		resp, err := client.execute(t.Context(), http.MethodGet, "/test", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("request creation error with invalid URL", func(t *testing.T) {
		client := NewClient(
			WithHTTPClient(&http.Client{}),
			WithBaseURL("://invalid-url"),
		)

		resp, err := client.execute(t.Context(), http.MethodGet, "/test", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "creating request")
		assert.Nil(t, resp)
	})

	t.Run("HTTP client error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		client := newTestClient(server)
		resp, err := client.execute(t.Context(), http.MethodGet, "/test", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "executing request")
		assert.Nil(t, resp)
	})

	t.Run("status 400 returns error with body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "bad request"}`))
		}))
		defer server.Close()

		client := newTestClient(server)
		resp, err := client.execute(t.Context(), http.MethodGet, "/test", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "API error 400")
		assert.ErrorContains(t, err, "bad request")
		assert.Nil(t, resp)
	})

	t.Run("status 500 returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		client := newTestClient(server)
		resp, err := client.execute(t.Context(), http.MethodGet, "/test", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "API error 500")
		assert.Nil(t, resp)
	})

	t.Run("large error body is truncated", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			largeBody := strings.Repeat("x", 8192)
			w.Write([]byte(largeBody))
		}))
		defer server.Close()

		client := newTestClient(server)
		resp, err := client.execute(t.Context(), http.MethodGet, "/test", nil)
		require.Error(t, err)

		assert.ErrorContains(t, err, "API error 400")
		assert.Less(t, len(err.Error()), 5000)
		assert.Nil(t, resp)
	})
}

func TestClient_buildURL(t *testing.T) {
	client := NewClient(WithBaseURL("https://api.example.com"))

	t.Run("path already has baseURL prefix", func(t *testing.T) {
		result := client.buildURL("https://api.example.com/items", nil)
		assert.Equal(t, "https://api.example.com/items", result)
	})

	t.Run("path without prefix no query", func(t *testing.T) {
		result := client.buildURL("/items", nil)
		assert.Equal(t, "https://api.example.com/items", result)
	})

	t.Run("path without prefix with query", func(t *testing.T) {
		query := url.Values{"page": {"1"}, "limit": {"10"}}
		result := client.buildURL("/items", query)
		assert.Contains(t, result, "https://api.example.com/items?")
		assert.Contains(t, result, "page=1")
		assert.Contains(t, result, "limit=10")
	})

	t.Run("empty query", func(t *testing.T) {
		result := client.buildURL("/items", url.Values{})
		assert.Equal(t, "https://api.example.com/items", result)
	})
}

func TestCollectResults(t *testing.T) {
	t.Run("all pages collected successfully", func(t *testing.T) {
		firstPage := itemsToRawMessages(makeTestItems(1, 3, 2025))
		resultsCh := make(chan pageResult, 2)
		resultsCh <- pageResult{pageNum: 2, items: itemsToRawMessages(makeTestItems(4, 3, 2025))}
		resultsCh <- pageResult{pageNum: 3, items: itemsToRawMessages(makeTestItems(7, 3, 2025))}
		close(resultsCh)

		results, err := collectResults(t.Context(), resultsCh, firstPage, 3, 2025)

		require.NoError(t, err)
		assert.Len(t, results, 9)
	})

	t.Run("captures first error only", func(t *testing.T) {
		firstPage := itemsToRawMessages(makeTestItems(1, 3, 2025))
		resultsCh := make(chan pageResult, 2)
		resultsCh <- pageResult{pageNum: 2, err: fmt.Errorf("error on page 2")}
		resultsCh <- pageResult{pageNum: 3, err: fmt.Errorf("error on page 3")}
		close(resultsCh)

		results, err := collectResults(t.Context(), resultsCh, firstPage, 3, 2025)
		require.Error(t, err)

		assert.ErrorContains(t, err, "page 2")
		assert.Nil(t, results)
	})

	t.Run("context error takes precedence", func(t *testing.T) {
		firstPage := itemsToRawMessages(makeTestItems(1, 3, 2025))
		resultsCh := make(chan pageResult, 1)
		resultsCh <- pageResult{pageNum: 2, err: fmt.Errorf("page error")}
		close(resultsCh)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		results, err := collectResults(ctx, resultsCh, firstPage, 2, 2025)
		require.Error(t, err)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, results)
	})

	t.Run("mixed success and error", func(t *testing.T) {
		firstPage := itemsToRawMessages(makeTestItems(1, 3, 2025))
		resultsCh := make(chan pageResult, 2)
		resultsCh <- pageResult{pageNum: 2, items: itemsToRawMessages(makeTestItems(4, 3, 2025))}
		resultsCh <- pageResult{pageNum: 3, err: fmt.Errorf("page error")}
		close(resultsCh)

		results, err := collectResults(t.Context(), resultsCh, firstPage, 3, 2025)
		require.Error(t, err)

		assert.ErrorContains(t, err, "page error")
		assert.Nil(t, results)
	})
}

func TestAssembleFilteredResults(t *testing.T) {
	t.Run("all pages present and in year", func(t *testing.T) {
		pages := map[int][]json.RawMessage{
			1: itemsToRawMessages(makeTestItems(1, 5, 2025)),
			2: itemsToRawMessages(makeTestItems(6, 5, 2025)),
			3: itemsToRawMessages(makeTestItems(11, 5, 2025)),
		}

		results := assembleFilteredResults(pages, 3, 2025)
		assert.Len(t, results, 15)
	})

	t.Run("missing page breaks early", func(t *testing.T) {
		pages := map[int][]json.RawMessage{
			1: itemsToRawMessages(makeTestItems(1, 5, 2025)),
			3: itemsToRawMessages(makeTestItems(11, 5, 2025)),
		}

		results := assembleFilteredResults(pages, 3, 2025)
		assert.Len(t, results, 5)
	})

	t.Run("year boundary causes early termination", func(t *testing.T) {
		pages := map[int][]json.RawMessage{
			1: itemsToRawMessages(makeTestItems(1, 5, 2025)),
			2: itemsToRawMessages([]testItem{
				makeTestItem(6, 2025),
				makeTestItem(7, 2024),
				makeTestItem(8, 2024),
			}),
			3: itemsToRawMessages(makeTestItems(11, 5, 2025)),
		}

		results := assembleFilteredResults(pages, 3, 2025)
		assert.Len(t, results, 6)
	})

	t.Run("empty pages map", func(t *testing.T) {
		pages := map[int][]json.RawMessage{}
		results := assembleFilteredResults(pages, 3, 2025)
		assert.Empty(t, results)
	})
}

func TesWithPage(t *testing.T) {
	t.Run("nil query creates new values with page", func(t *testing.T) {
		result := withPage(nil, 5)

		assert.Equal(t, "5", result.Get("page"))
		assert.Len(t, result, 1)
	})

	t.Run("preserves existing query params", func(t *testing.T) {
		query := url.Values{
			"filter": {"active"},
			"sort":   {"date"},
		}

		result := withPage(query, 3)

		assert.Equal(t, "3", result.Get("page"))
		assert.Equal(t, "active", result.Get("filter"))
		assert.Equal(t, "date", result.Get("sort"))
	})

	t.Run("does not modify original query", func(t *testing.T) {
		query := url.Values{"filter": {"active"}}
		original := query.Encode()

		_ = withPage(query, 5)

		assert.Equal(t, original, query.Encode())
		assert.Empty(t, query.Get("page"))
	})

	t.Run("overwrites existing page param", func(t *testing.T) {
		query := url.Values{"page": {"1"}}

		result := withPage(query, 10)

		assert.Equal(t, "10", result.Get("page"))
	})
}

func TestCalculateTotalPages(t *testing.T) {
	tests := []struct {
		name       string
		totalItems int
		expected   int
	}{
		{"zero items", 0, 0},
		{"negative items", -5, 0},
		{"1 item (page 1)", 1, 1},
		{"15 items (page 1 max)", 15, 1},
		{"16 items (page 2)", 16, 2},
		{"45 items (page 2 max)", 45, 2},
		{"46 items (page 3)", 46, 3},
		{"95 items (page 3 max)", 95, 3},
		{"96 items (page 4)", 96, 4},
		{"195 items", 195, 4},
		{"196 items", 196, 5},
		{"295 items", 295, 5},
		{"1000 items", 1000, 13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTotalPages(tt.totalItems)
			assert.Equal(t, tt.expected, result, "totalItems=%d", tt.totalItems)
		})
	}
}

func TestFilterItemsByYear(t *testing.T) {
	t.Run("empty items", func(t *testing.T) {
		result := filterItemsByYear([]json.RawMessage{}, 2025)
		assert.Empty(t, result)
	})

	t.Run("all items in target year", func(t *testing.T) {
		items := itemsToRawMessages(makeTestItems(1, 5, 2025))
		result := filterItemsByYear(items, 2025)
		assert.Len(t, result, 5)
	})

	t.Run("no items in target year", func(t *testing.T) {
		items := itemsToRawMessages(makeTestItems(1, 5, 2022))
		result := filterItemsByYear(items, 2025)
		assert.Empty(t, result)
	})

	t.Run("mixed years with early termination", func(t *testing.T) {
		items := itemsToRawMessages([]testItem{
			makeTestItem(1, 2025),
			makeTestItem(2, 2025),
			makeTestItem(3, 2024),
			makeTestItem(4, 2024),
		})

		result := filterItemsByYear(items, 2025)
		assert.Len(t, result, 2)
	})

	t.Run("skips malformed items", func(t *testing.T) {
		items := []json.RawMessage{
			itemsToRawMessages([]testItem{makeTestItem(1, 2025)})[0],
			[]byte("invalid json"),
			itemsToRawMessages([]testItem{makeTestItem(2, 2025)})[0],
		}

		result := filterItemsByYear(items, 2025)
		assert.Len(t, result, 2)
	})

	t.Run("includes items from future years", func(t *testing.T) {
		items := itemsToRawMessages([]testItem{
			makeTestItem(1, 2025),
			makeTestItem(2, 2025),
		})

		result := filterItemsByYear(items, 2025)
		assert.Len(t, result, 2)
	})
}

func TestClient_tracker(t *testing.T) {
	t.Run("full pagination flow records complete trace", func(t *testing.T) {
		handler := newMockPaginatedHandler()
		handler.totalItems = 100

		handler.setPage(1, makeTestItemsSameYear(1, 15, 2025))
		handler.setPage(2, makeTestItemsSameYear(16, 30, 2025))
		handler.setPage(3, makeTestItemsSameYear(46, 50, 2025))
		handler.setPage(4, makeTestItemsSameYear(96, 5, 2025))

		server := httptest.NewServer(handler)
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()
		assert.NotNil(t, tracker)

		results, err := client.GetPaginated(ctx, "/items", nil, 2025)
		require.NoError(t, err)

		assert.NotEmpty(t, results)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)

		assert.NotNil(t, state.Pagination)
		assert.Equal(t, state.Pagination.LastPageFound, 4)
		assert.NotNil(t, state.Filter)
		assert.Equal(t, 100, state.Filter.TotalItems)
		assert.Nil(t, state.Error)
	})

	t.Run("error flow records failure in trace", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount == 1 {
				w.Header().Set("X-Total-Count", "100")
				json.NewEncoder(w).Encode(makeTestItemsSameYear(1, 15, 2025))
				return
			}
			http.Error(w, "error", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, tracker := contextWithTracking()

		_, err := client.GetPaginated(ctx, "/items", nil, 2025)
		require.Error(t, err)

		state, exists := tracker.Snapshot().GetTrace(testResourceType)
		require.True(t, exists)
		assert.Equal(t, trace.StatusError, state.Status)
		assert.NotNil(t, state.Error)
	})
}

type testItem struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// makeTestItem creates a test item with specified year
func makeTestItem(id int, year int) testItem {
	return testItem{
		ID:        id,
		Title:     fmt.Sprintf("Item %d", id),
		CreatedAt: time.Date(year, 6, 15, 12, 0, 0, 0, time.UTC),
	}
}

// makeTestItems creates n items starting from startYear, decrementing year every 10 items.
// Useful for creating lists that span multiple years.
func makeTestItems(startID, count, startYear int) []testItem {
	items := make([]testItem, count)
	for i := range count {
		year := startYear - (i / 10) // Change year every 10 items
		items[i] = makeTestItem(startID+i, year)
	}
	return items
}

// makeTestItemsSameYear creates n items ALL from the same year
func makeTestItemsSameYear(startID, count, year int) []testItem {
	items := make([]testItem, count)
	for i := range count {
		items[i] = makeTestItem(startID+i, year)
	}
	return items
}

// itemsToRawMessages converts testItems to json.RawMessage slice
func itemsToRawMessages(items []testItem) []json.RawMessage {
	result := make([]json.RawMessage, len(items))
	for i, item := range items {
		data, _ := json.Marshal(item)
		result[i] = data
	}
	return result
}

// newTestClient creates a client configured for testing with the given server
func newTestClient(server *httptest.Server) *Client {
	return NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithAccessToken("test-token"),
		WithUserAgent("test-agent"),
		WithConcurrencyLimit(20),
	)
}

// newTestClientWithSemaphore creates a client with custom semaphore size
func newTestClientWithSemaphore(server *httptest.Server, size int) *Client {
	client := newTestClient(server)
	client.semaphore = make(chan struct{}, size)
	return client
}

// mockPaginatedHandler creates a handler that serves paginated responses
type mockPaginatedHandler struct {
	mu          sync.Mutex
	pages       map[int][]testItem
	totalItems  int
	failOnPage  int
	delayOnPage int
	delay       time.Duration
}

func newMockPaginatedHandler() *mockPaginatedHandler {
	return &mockPaginatedHandler{
		pages:      make(map[int][]testItem),
		failOnPage: -1,
	}
}

func (h *mockPaginatedHandler) setPage(pageNum int, items []testItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pages[pageNum] = items
}

func (h *mockPaginatedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page == 0 {
		page = 1
	}

	if h.failOnPage == page {
		pageStr := strconv.Itoa(page)
		http.Error(w, "simulated fetch error on page "+pageStr, http.StatusInternalServerError)
		return
	}

	if h.delayOnPage == page && h.delay > 0 {
		time.Sleep(h.delay)
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(h.totalItems))
	w.Header().Set("Content-Type", "application/json")

	items, ok := h.pages[page]
	if !ok {
		items = []testItem{}
	}

	json.NewEncoder(w).Encode(items)
}

// contextWithTracking creates a context with real tracking enabled
func contextWithTracking() (context.Context, *trace.Tracer) {
	tracker := trace.NewTracer()
	tracker.RegisterTrace(testResourceType)

	ctx := trace.WithTracker(context.Background(), tracker)
	ctx = trace.WithResourceType(ctx, testResourceType)

	return ctx, tracker
}

func assertResultsEqual(t *testing.T, expected any, actual any) {
	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)
	assert.JSONEq(t, string(expectedJSON), string(actualJSON))
}
