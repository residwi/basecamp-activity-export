package basecamp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/httpcache"
	"github.com/residwi/basecamp-activity-export/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestMiddleware_Tracking(t *testing.T) {
	const stubResourceType trace.ResourceType = "stub-resource"
	targetURL := "http://api.com/users?id=1"

	t.Run("tracks success (200)", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{mockResp(200, "OK", nil)},
		}

		tracker := trace.NewTracer()
		tracker.RegisterTrace(stubResourceType)
		client := ChainTransport(transport, Tracking())

		ctx := context.Background()
		ctx = trace.WithTracker(ctx, tracker)
		ctx = trace.WithResourceType(ctx, stubResourceType)

		req, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)

		_, err := client.RoundTrip(req)
		require.NoError(t, err)

		traceState, exists := tracker.Snapshot().GetTrace(stubResourceType)
		require.True(t, exists, "Trace state was not found for resource type")
		require.NotNil(t, traceState.Network, "Network stats were not initialized")

		reqInfo, ok := traceState.Network.Requests[targetURL]
		require.True(t, ok, "Request URL was not tracked. Expected key: %s", targetURL)

		assert.Equal(t, 200, reqInfo.StatusCode)
		assert.False(t, reqInfo.EndTime.Before(reqInfo.StartTime), "EndTime recorded before StartTime")
	})

	t.Run("tracks 304 as cached", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{mockResp(304, "", nil)},
		}

		tracker := trace.NewTracer()
		tracker.RegisterTrace(stubResourceType)
		client := ChainTransport(transport, Tracking())

		ctx := context.Background()
		ctx = trace.WithTracker(ctx, tracker)
		ctx = trace.WithResourceType(ctx, stubResourceType)
		req, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)

		client.RoundTrip(req)

		traceState, exists := tracker.Snapshot().GetTrace(stubResourceType)
		require.True(t, exists, "Trace state was not found for resource type")
		require.NotNil(t, traceState.Network)

		reqInfo, ok := traceState.Network.Requests[targetURL]
		require.True(t, ok, "Request URL was not tracked")

		assert.Equal(t, 304, reqInfo.StatusCode)
		assert.True(t, reqInfo.Cached, "Expected request to be marked as Cached=true for 304 response")
	})

	t.Run("tracks network error as status 0", func(t *testing.T) {
		transport := &MockTransport{
			Errors: []error{errors.New("connection reset")},
		}

		tracker := trace.NewTracer()
		tracker.RegisterTrace(stubResourceType)
		client := ChainTransport(transport, Tracking())

		ctx := context.Background()
		ctx = trace.WithTracker(ctx, tracker)
		ctx = trace.WithResourceType(ctx, stubResourceType)
		req, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)

		client.RoundTrip(req)

		traceState, exists := tracker.Snapshot().GetTrace(stubResourceType)
		require.True(t, exists, "Trace state was not found for resource type")

		reqInfo := traceState.Network.Requests[targetURL]
		assert.Equal(t, 0, reqInfo.StatusCode, "Expected status 0 for network failure")
	})

	t.Run("skips when no tracker in context", func(t *testing.T) {
		transport := &MockTransport{Responses: []*http.Response{mockResp(200, "OK", nil)}}
		client := ChainTransport(transport, Tracking())

		req, _ := http.NewRequest("GET", targetURL, nil)

		client.RoundTrip(req)

		assert.Equal(t, 1, transport.CallCount, "Request should have proceeded without tracking")
	})
}

func TestMiddleware_Cache(t *testing.T) {
	targetURL := "http://api.com/1"

	t.Run("cache miss - stores data", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{
				mockResp(200, `{"id":1}`, map[string]string{"ETag": "v1"}),
			},
		}

		tmpDir := t.TempDir()
		cacheManager, _ := httpcache.NewFileManager(tmpDir)
		client := ChainTransport(transport, Cache(cacheManager))

		req, _ := http.NewRequest("GET", targetURL, nil)
		resp, err := client.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		assert.JSONEq(t, `{"id":1}`, string(body), "Response body mismatch")

		entry, err := cacheManager.Get(context.Background(), targetURL)
		require.NoError(t, err, "Expected cache entry to exist")

		assert.Equal(t, "v1", entry.ETag)
		assert.JSONEq(t, `{"id":1}`, string(entry.Body), "Expected cached body content")
	})

	t.Run("returns 304 - serves from cache", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheManager, _ := httpcache.NewFileManager(tmpDir)

		err := cacheManager.Set(context.Background(), targetURL, "v1", "stub-last-modified", []byte(`{"id":1}`))
		require.NoError(t, err)

		transport := &MockTransport{
			Responses: []*http.Response{
				mockResp(304, "", nil),
			},
		}
		client := ChainTransport(transport, Cache(cacheManager))

		req, _ := http.NewRequest("GET", targetURL, nil)
		resp, err := client.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Len(t, transport.Requests, 1, "Expected network request")
		assert.Equal(t, "v1", transport.Requests[0].Header.Get("If-None-Match"), "Middleware failed to inject ETag")
		assert.Equal(t, "stub-last-modified", transport.Requests[0].Header.Get("If-Modified-Since"), "Middleware failed to inject Last-Modified")

		body, _ := io.ReadAll(resp.Body)
		assert.JSONEq(t, `{"id":1}`, string(body), "Expected restored body from cache")
		assert.Equal(t, 304, resp.StatusCode)
	})

	t.Run("cache hit - returns 200 - updates cache", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{
				mockResp(200, `{"id":2}`, map[string]string{"ETag": "v2"}),
			},
		}

		tmpDir := t.TempDir()
		cacheManager, _ := httpcache.NewFileManager(tmpDir)
		client := ChainTransport(transport, Cache(cacheManager))

		req, _ := http.NewRequest("GET", "http://api.com/1", nil)
		resp, _ := client.RoundTrip(req)
		resp.Body.Close()

		entry, err := cacheManager.Get(context.Background(), "http://api.com/1")
		require.NoError(t, err, "Expected cache entry to exist after update")

		assert.Equal(t, "v2", entry.ETag)
		assert.JSONEq(t, `{"id":2}`, string(entry.Body), "Expected updated cached body")
	})

	t.Run("non-GET method ignored", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{
				mockResp(200, "Posted", nil),
			},
		}

		tmpDir := t.TempDir()
		cacheManager, _ := httpcache.NewFileManager(tmpDir)
		client := ChainTransport(transport, Cache(cacheManager))

		req, _ := http.NewRequest("POST", "http://api.com/1", nil)
		client.RoundTrip(req)

		entry, err := cacheManager.Get(context.Background(), "http://api.com/1")

		assert.Error(t, err, "Expected error when cache entry doesn't exist")
		assert.ErrorIs(t, err, os.ErrNotExist, "Expected ErrNotExist for non-existent cache file")
		assert.Nil(t, entry, "Entry should be nil when cache doesn't exist")
	})

	t.Run("returns network error", func(t *testing.T) {
		transport := &MockTransport{
			Errors: []error{errors.New("network failure")},
		}

		tmpDir := t.TempDir()
		cacheManager, _ := httpcache.NewFileManager(tmpDir)
		client := ChainTransport(transport, Cache(cacheManager))

		req, _ := http.NewRequest("GET", targetURL, nil)
		_, err := client.RoundTrip(req)

		assert.Error(t, err, "Expected network error to propagate")
	})

	t.Run("failed to store cache - continues gracefully", func(t *testing.T) {
		header := make(http.Header)
		header.Set("ETag", strings.Repeat("a", 300))

		transport := &MockTransport{
			Responses: []*http.Response{
				{
					StatusCode: 200,
					Header:     header,
					Body:       &mockReadCloser{err: errors.New("read error")},
				},
			},
		}

		tmpDir := t.TempDir()
		cacheManager, _ := httpcache.NewFileManager(tmpDir)
		client := ChainTransport(transport, Cache(cacheManager))

		req, _ := http.NewRequest("GET", targetURL, nil)
		resp, err := client.RoundTrip(req)
		require.Error(t, err, "Expected read error to propagate")
		require.Nil(t, resp, "Expected HTTP response despite cache store failure")

		assert.EqualError(t, err, "reading response: read error")
		assert.Equal(t, 1, transport.CallCount, "Expected single network call")
	})
}

func TestMiddleware_Retry(t *testing.T) {
	const stubResourceType trace.ResourceType = "stub-resource"
	tracker := trace.NewTracer()
	tracker.RegisterTrace(stubResourceType)
	ctx := context.Background()
	ctx = trace.WithTracker(ctx, tracker)
	ctx = trace.WithResourceType(ctx, stubResourceType)

	t.Run("success on first try", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{mockResp(200, "OK", nil)},
		}
		client := ChainTransport(transport, Retry(3))
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.com", nil)

		resp, _ := client.RoundTrip(req)

		assert.Equal(t, 1, transport.CallCount)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("retry on 429 with retry-after", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := &MockTransport{
				Responses: []*http.Response{
					mockResp(429, "Busy", map[string]string{"Retry-After": "0"}),
					mockResp(200, "OK", nil),
				},
			}
			client := ChainTransport(transport, Retry(3))
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.com", nil)

			resp, _ := client.RoundTrip(req)

			assert.Equal(t, 2, transport.CallCount, "Expected 2 calls (1 retry)")
			assert.Equal(t, 200, resp.StatusCode, "Expected final status 200")
		})
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := &MockTransport{
				Responses: []*http.Response{
					mockResp(500, "Err", nil),
					mockResp(500, "Err", nil),
					mockResp(500, "Err", nil),
					mockResp(500, "Err", nil),
				},
			}
			client := ChainTransport(transport, Retry(3))
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.com", nil)

			resp, err := client.RoundTrip(req)
			require.NoError(t, err, "Expected HTTP response, even on failure")

			assert.Equal(t, 4, transport.CallCount, "Expected 4 calls")
			assert.Equal(t, 500, resp.StatusCode, "Expected final status 500")
		})
	})

	t.Run("max exponential backoff cap", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var responses []*http.Response
			for range 10 {
				responses = append(responses, mockResp(500, "Err", nil))
			}
			responses = append(responses, mockResp(200, "OK", nil))

			transport := &MockTransport{Responses: responses}
			client := ChainTransport(transport, Retry(10))
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.com", nil)

			start := time.Now()
			resp, _ := client.RoundTrip(req)
			elapsed := time.Since(start)

			assert.Equal(t, 11, transport.CallCount, "Expected 11 calls (10 retries)")
			assert.Equal(t, 200, resp.StatusCode, "Expected final status 200")

			// exponential backoff with cap should keep total wait time under 3 minutes
			assert.Less(t, elapsed, 3*time.Minute, "Total retry wait time exceeded expected maximum")
		})
	})

	t.Run("do not retry POST on 500 (non-idempotent)", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{mockResp(500, "Err", nil)},
		}
		client := ChainTransport(transport, Retry(3))

		req, _ := http.NewRequestWithContext(ctx, "POST", "http://api.com", bytes.NewBufferString("{}"))

		client.RoundTrip(req)

		assert.Equal(t, 1, transport.CallCount, "Should not retry POST on 500")
	})

	t.Run("context cancellation stops retry", func(t *testing.T) {
		transport := &MockTransport{
			Responses: []*http.Response{mockResp(500, "Err", nil)},
		}
		client := ChainTransport(transport, Retry(10))

		ctx, cancel := context.WithCancel(ctx)
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.com", nil)

		cancel()

		_, err := client.RoundTrip(req)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("retry rewinds body", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := &MockTransport{
				Responses: []*http.Response{
					mockResp(500, "Err", nil),
					mockResp(200, "OK", nil),
				},
			}
			client := ChainTransport(transport, Retry(1))

			bodyContent := "payload"
			req, _ := http.NewRequestWithContext(ctx, "PUT", "http://api.com", strings.NewReader(bodyContent))

			client.RoundTrip(req)

			require.Len(t, transport.Requests, 2, "Expected 2 requests")

			lastReq := transport.Requests[1]
			sentBody, _ := io.ReadAll(lastReq.Body)
			assert.Equal(t, bodyContent, string(sentBody), "Body was not rewound correctly")
		})
	})

	t.Run("retry fails to rewind body", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := &MockTransport{
				Responses: []*http.Response{
					mockResp(500, "Err", nil),
					mockResp(200, "OK", nil),
				},
			}
			client := ChainTransport(transport, Retry(1))

			req, _ := http.NewRequestWithContext(ctx, "PUT", "http://api.com", nil)
			req.GetBody = func() (io.ReadCloser, error) {
				return nil, errors.New("cannot rewind body")
			}
			_, err := client.RoundTrip(req)
			require.Error(t, err, "Expected error due to body rewind failure")
			assert.Equal(t, 1, transport.CallCount, "Expected only 1 request due to rewind failure")
		})
	})

	t.Run("do not retry streaming body", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			transport := &MockTransport{}
			client := ChainTransport(transport, Retry(1))

			req, _ := http.NewRequestWithContext(ctx, "PUT", "http://api.com", nil)
			req.Body = io.NopCloser(&mockReadCloser{err: errors.New("streaming body error")})
			req.GetBody = nil // indicates streaming body

			_, err := client.RoundTrip(req)
			require.Error(t, err, "Expected error due to streaming body on retry")

			assert.Equal(t, 1, transport.CallCount, "Expected only 1 request for streaming body")
		})
	})
}

func TestMiddleware_RateLimit(t *testing.T) {
	t.Run("blocks when rate limit exceeded", func(t *testing.T) {
		transport := &MockTransport{Responses: []*http.Response{mockResp(200, "OK", nil)}}

		limiter := rate.NewLimiter(rate.Every(1*time.Hour), 0) // burst 0, blocked immediately
		client := ChainTransport(transport, RateLimit(limiter))
		req, _ := http.NewRequest("GET", "http://api.com", nil)

		_, err := client.RoundTrip(req)

		require.Error(t, err)
		assert.Equal(t, 0, transport.CallCount, "Request should have been blocked by rate limiter")
	})

	t.Run("blocks on context cancellation while waiting", func(t *testing.T) {
		transport := &MockTransport{Responses: []*http.Response{mockResp(200, "OK", nil)}}

		limiter := rate.NewLimiter(rate.Every(1*time.Hour), 1)
		client := ChainTransport(transport, RateLimit(limiter))

		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://api.com", nil)

		// Cancel the context immediately
		cancel()

		_, err := client.RoundTrip(req)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, transport.CallCount, "Request should have been blocked by rate limiter")
	})

	t.Run("allows request when token available", func(t *testing.T) {
		transport := &MockTransport{Responses: []*http.Response{mockResp(200, "OK", nil)}}
		limiter := rate.NewLimiter(rate.Inf, 1)
		client := ChainTransport(transport, RateLimit(limiter))

		req, _ := http.NewRequest("GET", "http://api.com", nil)
		_, err := client.RoundTrip(req)
		assert.NoError(t, err)

		assert.Equal(t, 1, transport.CallCount, "Request should have passed through")
	})
}

func TestMiddleware_ChainTransport_Order(t *testing.T) {
	var executionTrace string

	middleware1 := func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			executionTrace += "1-Start "
			resp, err := next.RoundTrip(req)
			executionTrace += "1-End "
			return resp, err
		})
	}
	middleware2 := func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			executionTrace += "2-Start "
			resp, err := next.RoundTrip(req)
			executionTrace += "2-End "
			return resp, err
		})
	}

	transport := &MockTransport{Responses: []*http.Response{mockResp(200, "OK", nil)}}

	// chain: 1 -> 2 -> Transport
	client := ChainTransport(transport, middleware1, middleware2)

	client.RoundTrip(&http.Request{})

	expected := "1-Start 2-Start 2-End 1-End "
	assert.Equal(t, expected, executionTrace, "Execution order wrong")
}

func TestMiddleware_ParseRetryAfter(t *testing.T) {
	t.Run("with seconds", func(t *testing.T) {
		duration := parseRetryAfter("5")
		assert.Equal(t, 5*time.Second, duration, "Failed to parse seconds")
	})

	t.Run("with HTTP Date", func(t *testing.T) {
		future := time.Now().UTC().Add(10 * time.Second).Format(http.TimeFormat)
		duration := parseRetryAfter(future)
		assert.Greater(t, duration, time.Duration(0), "Failed to parse HTTP Date")
		assert.LessOrEqual(t, duration, 11*time.Second, "Failed to parse HTTP Date")
	})

	t.Run("with invalid input", func(t *testing.T) {
		duration := parseRetryAfter("invalid")
		assert.Equal(t, time.Duration(0), duration, "Expected 0 for invalid input")
	})

	t.Run("with past date", func(t *testing.T) {
		past := time.Now().UTC().Add(-10 * time.Hour).Format(http.TimeFormat)
		duration := parseRetryAfter(past)
		assert.Equal(t, time.Duration(0), duration, "Expected 0 for past date")
	})
}

// MockTransport simulates a server. It returns a sequence of responses or errors.
type MockTransport struct {
	mu        sync.Mutex
	Responses []*http.Response // Stack of responses to return
	Errors    []error          // Stack of errors to return (overrides Responses if present at index)
	CallCount int
	Requests  []*http.Request // History of received requests
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++
	m.Requests = append(m.Requests, req)

	// Return Error if defined for this call
	if len(m.Errors) >= m.CallCount {
		if err := m.Errors[m.CallCount-1]; err != nil {
			return nil, err
		}
	}

	// Return Response
	if len(m.Responses) >= m.CallCount {
		resp := m.Responses[m.CallCount-1]
		// Mock the Body ReadCloser behavior
		if resp.Body == nil {
			resp.Body = http.NoBody
		}
		return resp, nil
	}

	return nil, errors.New("mock transport: no more responses configured")
}

// Helper to create a fake HTTP Response
func mockResp(code int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp
}

// mockReadCloser helps simulate reader errors
type mockReadCloser struct {
	err error
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) { return 0, m.err }
func (m *mockReadCloser) Close() error                     { return nil }
