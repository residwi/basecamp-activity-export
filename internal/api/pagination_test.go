package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/residwi/bcreview/internal/cache"
	"github.com/residwi/bcreview/internal/tracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPageData generates mock data for a specific page and year
func mockPageData(pageNum, itemsPerPage, year int) []map[string]any {
	data := make([]map[string]any, itemsPerPage)
	for i := range itemsPerPage {
		// Items are sorted DESC by created_at (newest first)
		// So page 1 has newest items, last page has oldest items
		daysAgo := (pageNum-1)*itemsPerPage + i
		createdAt := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC).AddDate(0, 0, -daysAgo)

		data[i] = map[string]any{
			"id":         (pageNum-1)*itemsPerPage + i + 1,
			"title":      fmt.Sprintf("Item %d", (pageNum-1)*itemsPerPage+i+1),
			"created_at": createdAt.Format(time.RFC3339),
		}
	}
	return data
}

// setupMockServer creates a test server that simulates Basecamp pagination
func setupMockServer(t *testing.T, totalPages, itemsPerPage, targetYear int) (*httptest.Server, *Client) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			page = 1
		}

		// Simulate empty pages beyond totalPages
		if page > totalPages {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		// Generate mock data for this page
		data := mockPageData(page, itemsPerPage, targetYear)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	}))

	tempDir := t.TempDir()
	cacheManager, _ := cache.NewFileManager(tempDir)
	client := NewClient("test-account", "test-token", cacheManager)
	client.baseURL = server.URL

	return server, client
}

func TestProbePage(t *testing.T) {
	t.Run("basic probing scenarios", func(t *testing.T) {
		tests := []struct {
			name            string
			pageNum         int
			targetYear      int
			totalPages      int
			itemsPerPage    int
			expectedHasData bool
			expectedHasYear bool
		}{
			{
				name:            "first page with target year data",
				pageNum:         1,
				targetYear:      2024,
				totalPages:      5,
				itemsPerPage:    10,
				expectedHasData: true,
				expectedHasYear: true,
			},
			{
				name:            "middle page with target year data",
				pageNum:         3,
				targetYear:      2024,
				totalPages:      5,
				itemsPerPage:    10,
				expectedHasData: true,
				expectedHasYear: true,
			},
			{
				name:            "page beyond total pages (empty)",
				pageNum:         10,
				targetYear:      2024,
				totalPages:      5,
				itemsPerPage:    10,
				expectedHasData: false,
				expectedHasYear: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server, client := setupMockServer(t, tt.totalPages, tt.itemsPerPage, tt.targetYear)
				defer server.Close()

				ctx := context.Background()
				result := client.probePage(ctx, "/recordings.json", nil, tt.targetYear, tt.pageNum)

				assert.NoError(t, result.err)
				assert.Equal(t, tt.pageNum, result.pageNum)
				assert.Equal(t, tt.expectedHasData, result.hasData, "hasData mismatch")
				assert.Equal(t, tt.expectedHasYear, result.hasTargetYear, "hasTargetYear mismatch")
			})
		}
	})

	t.Run("year boundary detection", func(t *testing.T) {
		// Create server that returns 2023 data when requesting 2024
		server, client := setupMockServer(t, 5, 10, 2023) // Server has 2023 data
		defer server.Close()

		ctx := context.Background()
		result := client.probePage(ctx, "/recordings.json", nil, 2024, 5) // Request 2024

		assert.NoError(t, result.err)
		assert.Equal(t, 5, result.pageNum)
		assert.True(t, result.hasData, "Should have data")
		assert.False(t, result.hasTargetYear, "2023 < 2024, so no target year data")
	})

	t.Run("with query parameters", func(t *testing.T) {
		server, client := setupMockServer(t, 5, 10, 2024)
		defer server.Close()

		ctx := context.Background()
		query := url.Values{}
		query.Set("status", "active")
		query.Set("bucket", "123")

		result := client.probePage(ctx, "/recordings.json", query, 2024, 1)

		assert.NoError(t, result.err)
		assert.True(t, result.hasData)
		assert.Equal(t, 1, result.pageNum)
	})
}

func TestFindLastPage(t *testing.T) {
	t.Run("exponential probing scenarios", func(t *testing.T) {
		tests := []struct {
			name           string
			totalPages     int
			itemsPerPage   int
			targetYear     int
			expectedResult int
		}{
			{
				name:           "small dataset - 2 pages",
				totalPages:     2,
				itemsPerPage:   10,
				targetYear:     2024,
				expectedResult: 2,
			},
			{
				name:           "medium dataset - 10 pages",
				totalPages:     10,
				itemsPerPage:   10,
				targetYear:     2024,
				expectedResult: 10,
			},
			{
				name:           "large dataset - 30 pages",
				totalPages:     30,
				itemsPerPage:   10,
				targetYear:     2024,
				expectedResult: 30,
			},
			{
				name:           "exactly at exponential boundary - 8 pages",
				totalPages:     8,
				itemsPerPage:   10,
				targetYear:     2024,
				expectedResult: 8,
			},
			{
				name:           "just after exponential boundary - 9 pages",
				totalPages:     9,
				itemsPerPage:   10,
				targetYear:     2024,
				expectedResult: 9,
			},
			{
				name:           "single page only",
				totalPages:     1,
				itemsPerPage:   10,
				targetYear:     2024,
				expectedResult: 1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server, client := setupMockServer(t, tt.totalPages, tt.itemsPerPage, tt.targetYear)
				defer server.Close()

				ctx := context.Background()
				lastPage, err := client.findLastPage(ctx, "/recordings.json", nil, tt.targetYear)

				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, lastPage, "Last page number mismatch")
			})
		}
	})

	t.Run("binary search refinement", func(t *testing.T) {
		// Test that binary search correctly refines between exponential probe boundaries
		// For example, if probes find lastGood=8, upperBound=16, binary search should find exact page
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageStr := r.URL.Query().Get("page")
			page, _ := strconv.Atoi(pageStr)
			if page == 0 {
				page = 1
			}

			var data []map[string]any

			// Pages 1-12 have data, pages 13+ are empty
			if page <= 12 {
				data = mockPageData(page, 10, 2024)
			} else {
				data = []map[string]any{}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		cacheManager, _ := cache.NewFileManager(tempDir)
		client := NewClient("test-account", "test-token", cacheManager)
		client.baseURL = server.URL

		ctx := context.Background()
		lastPage, err := client.findLastPage(ctx, "/recordings.json", nil, 2024)

		require.NoError(t, err)
		assert.Equal(t, 12, lastPage, "Binary search should find exact boundary at page 12")
	})

	t.Run("year boundary crossing", func(t *testing.T) {
		// Create a server with data spanning multiple years
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageStr := r.URL.Query().Get("page")
			page, _ := strconv.Atoi(pageStr)
			if page == 0 {
				page = 1
			}

			var data []map[string]any

			switch page {
			case 1, 2, 3:
				// Pages 1-3: 2024 data
				data = mockPageData(page, 10, 2024)
			case 4:
				// Page 4: All 2023 data (year boundary crossed)
				// probePage checks last item, which will be 2023
				data = make([]map[string]any, 10)
				for i := range 10 {
					data[i] = map[string]any{
						"id":         i + 1,
						"created_at": time.Date(2023, 12, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
					}
				}
			case 5, 6:
				// Pages 5-6: 2023 data (too old)
				data = mockPageData(page, 10, 2023)
			default:
				// Empty pages
				data = []map[string]any{}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		cacheManager, _ := cache.NewFileManager(tempDir)
		client := NewClient("test-account", "test-token", cacheManager)
		client.baseURL = server.URL

		ctx := context.Background()
		lastPage, err := client.findLastPage(ctx, "/recordings.json", nil, 2024)

		require.NoError(t, err)
		// Should find page 3 as the last page with 2024 data (page 4 has only 2023)
		assert.Equal(t, 3, lastPage, "Should stop at boundary page")
	})

	t.Run("with tracking enabled", func(t *testing.T) {
		server, client := setupMockServer(t, 10, 10, 2024)
		defer server.Close()

		tracker := tracking.NewTracker(true)
		ctx := tracking.WithTracker(context.Background(), tracker)
		ctx = tracking.WithResourceType(ctx, tracking.Messages)

		tracker.RegisterTrace(tracking.Messages)

		lastPage, err := client.findLastPage(ctx, "/recordings.json", nil, 2024)

		require.NoError(t, err)
		assert.Equal(t, 10, lastPage)

		// Verify tracking was recorded
		state, exists := tracker.GetTrace(tracking.Messages)
		assert.True(t, exists, "Should have tracking state")
		if exists {
			assert.NotEqual(t, tracking.StatusPending, state.Status, "Should have completed or progressed")
		}
	})

	t.Run("context cancellation handling", func(t *testing.T) {
		server, client := setupMockServer(t, 100, 10, 2024)
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Should handle cancellation gracefully (may return error or partial result)
		// The important thing is it doesn't hang or panic
		assert.NotPanics(t, func() {
			_, _ = client.findLastPage(ctx, "/recordings.json", nil, 2024)
		})
	})
}

func TestFetchPagesParallel(t *testing.T) {
	t.Run("year filtering scenarios", func(t *testing.T) {
		tests := []struct {
			name          string
			totalPages    int
			itemsPerPage  int
			targetYear    int
			lastPage      int
			expectedItems int
		}{
			{
				name:          "all pages within year",
				totalPages:    3,
				itemsPerPage:  10,
				targetYear:    2024,
				lastPage:      3,
				expectedItems: 30,
			},
			{
				name:          "single page",
				totalPages:    1,
				itemsPerPage:  10,
				targetYear:    2024,
				lastPage:      1,
				expectedItems: 10,
			},
			{
				name:          "no pages requested",
				totalPages:    5,
				itemsPerPage:  10,
				targetYear:    2024,
				lastPage:      0,
				expectedItems: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server, client := setupMockServer(t, tt.totalPages, tt.itemsPerPage, tt.targetYear)
				defer server.Close()

				ctx := context.Background()
				query := url.Values{}
				results, err := client.fetchPagesParallel(ctx, "/recordings.json", query, tt.targetYear, tt.lastPage)

				require.NoError(t, err)
				assert.Len(t, results, tt.expectedItems, "Item count mismatch")
			})
		}
	})

	t.Run("boundary page with mixed year data", func(t *testing.T) {
		// Create server with year boundary in the middle of a page
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageStr := r.URL.Query().Get("page")
			page, _ := strconv.Atoi(pageStr)
			if page == 0 {
				page = 1
			}

			var data []map[string]any

			if page == 1 {
				// Page 1: Mixed data - first 5 items from 2024, last 5 from 2023
				data = make([]map[string]any, 10)
				for i := range 5 {
					data[i] = map[string]any{
						"id":         i + 1,
						"created_at": time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
					}
				}
				for i := 5; i < 10; i++ {
					data[i] = map[string]any{
						"id":         i + 1,
						"created_at": time.Date(2023, 12, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
					}
				}
			} else {
				data = []map[string]any{}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
		}))
		defer server.Close()

		tempDir := t.TempDir()
		cacheManager, _ := cache.NewFileManager(tempDir)
		client := NewClient("test-account", "test-token", cacheManager)
		client.baseURL = server.URL

		ctx := context.Background()
		query := url.Values{}
		results, err := client.fetchPagesParallel(ctx, "/recordings.json", query, 2024, 1)

		require.NoError(t, err)
		// Should only get first 5 items (2024 data), stop when hitting 2023
		assert.Equal(t, 5, len(results), "Should filter out old year data")
	})

	t.Run("empty pages handling", func(t *testing.T) {
		server, client := setupMockServer(t, 0, 10, 2024)
		defer server.Close()

		ctx := context.Background()
		query := url.Values{}
		results, err := client.fetchPagesParallel(ctx, "/recordings.json", query, 2024, 3)

		require.NoError(t, err)
		assert.Empty(t, results, "Should handle empty pages gracefully")
	})

	t.Run("with tracking enabled", func(t *testing.T) {
		server, client := setupMockServer(t, 3, 10, 2024)
		defer server.Close()

		tracker := tracking.NewTracker(true)
		ctx := tracking.WithTracker(context.Background(), tracker)
		ctx = tracking.WithResourceType(ctx, tracking.Todos)

		tracker.RegisterTrace(tracking.Todos)

		query := url.Values{}
		results, err := client.fetchPagesParallel(ctx, "/recordings.json", query, 2024, 3)

		require.NoError(t, err)
		assert.Len(t, results, 30)

		// Verify tracking
		state, exists := tracker.GetTrace(tracking.Todos)
		assert.True(t, exists, "Should have tracking state")
		if exists {
			assert.NotNil(t, state.Network, "Should have network stats")
		}
	})
}

func TestGetPaginated(t *testing.T) {
	t.Run("integration scenarios", func(t *testing.T) {
		tests := []struct {
			name          string
			totalPages    int
			itemsPerPage  int
			targetYear    int
			expectedItems int
		}{
			{
				name:          "small dataset",
				totalPages:    2,
				itemsPerPage:  5,
				targetYear:    2024,
				expectedItems: 10,
			},
			{
				name:          "medium dataset",
				totalPages:    5,
				itemsPerPage:  10,
				targetYear:    2024,
				expectedItems: 50,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server, client := setupMockServer(t, tt.totalPages, tt.itemsPerPage, tt.targetYear)
				defer server.Close()

				tracker := tracking.NewTracker(true)
				ctx := tracking.WithTracker(context.Background(), tracker)
				ctx = tracking.WithResourceType(ctx, tracking.Messages)

				tracker.RegisterTrace(tracking.Messages)

				query := url.Values{}
				results, err := client.getPaginated(ctx, "/recordings.json", query, tt.targetYear)

				require.NoError(t, err)
				assert.Len(t, results, tt.expectedItems)

				// Verify tracking completed
				state, exists := tracker.GetTrace(tracking.Messages)
				assert.True(t, exists, "Should have tracking state")
				if exists {
					assert.NotNil(t, state.Pagination, "Should have pagination stats")
					assert.NotNil(t, state.Network, "Should have network stats")
				}
			})
		}
	})
}
