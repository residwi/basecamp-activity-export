package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/residwi/bcreview/internal/cache"
	"github.com/residwi/bcreview/internal/ratelimit"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	accountID := "123456"
	accessToken := "test-token"

	tempDir := t.TempDir()
	cacheManager, _ := cache.NewFileManager(tempDir)

	client := NewClient(accountID, accessToken, cacheManager)
	baseURL := BaseURL + "/" + accountID

	assert.NotNil(t, client)
	assert.Equal(t, accountID, client.accountID)
	assert.Equal(t, accessToken, client.accessToken)
	assert.Equal(t, baseURL, client.baseURL)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.rateLimiter)
	assert.NotNil(t, client.cache)
	assert.NotNil(t, client.semaphore)
	assert.Equal(t, UserAgent, client.userAgent)
}

func TestBuildRequest_BasicAuth(t *testing.T) {
	tempDir := t.TempDir()
	cacheManager, _ := cache.NewFileManager(tempDir)
	client := NewClient("123456", "test-token", cacheManager)

	req, err := http.NewRequest("GET", client.baseURL+"/projects.json", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+client.accessToken)
	req.Header.Set("User-Agent", client.userAgent)

	assert.NoError(t, err)
	assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
	assert.Equal(t, client.userAgent, req.Header.Get("User-Agent"))
}

func TestRateLimitHits_Counter(t *testing.T) {
	tempDir := t.TempDir()
	cacheManager, _ := cache.NewFileManager(tempDir)
	client := NewClient("123456", "test-token", cacheManager)

	if client.rateLimitHits.Load() != 0 {
		t.Error("Expected initial rateLimitHits to be 0")
	}

	client.rateLimitHits.Add(1)
	client.rateLimitHits.Add(1)
	client.rateLimitHits.Add(1)

	assert.Equal(t, uint64(3), client.rateLimitHits.Load())
}

func TestClient_RateLimiter_Integration(t *testing.T) {
	limiter := ratelimit.NewSlidingWindowLimiter(5, 1*time.Second)

	for i := range 5 {
		assert.True(t, limiter.Allow(), "Request %d should be allowed", i+1)
	}

	assert.False(t, limiter.Allow(), "Request 6 should be rate limited")
}

func TestClient_CacheManager_Integration(t *testing.T) {
	tempDir := t.TempDir()
	cacheManager, err := cache.NewFileManager(tempDir)
	assert.NoError(t, err, "Failed to create cache manager")

	client := NewClient("123456", "test-token", cacheManager)
	assert.NotNil(t, client.cache, "Cache manager should not be nil")

	ctx := context.Background()
	url := "https://api.example.com/test"
	body := []byte(`{"test": "data"}`)

	err = client.cache.Set(ctx, url, "etag1", "", body)
	assert.NoError(t, err, "Cache Set failed")

	entry, err := client.cache.Get(ctx, url)
	assert.NoError(t, err, "Cache Get failed")

	assert.NotNil(t, entry, "Cache entry should not be nil")
	assert.Equal(t, "etag1", string(entry.ETag), "ETag should match")
}

func TestClient_HTTPClientConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	cacheManager, _ := cache.NewFileManager(tempDir)
	client := NewClient("123456", "test-token", cacheManager)

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.httpClient.Timeout)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	assert.True(t, ok, "Expected http.Transport type")
	assert.NotNil(t, transport, "Transport should not be nil")

	assert.Equal(t, RateLimitRequests, transport.MaxIdleConns, "Expected MaxIdleConns 100")
	assert.Equal(t, RateLimitRequests, transport.MaxIdleConnsPerHost, "Expected MaxIdleConnsPerHost to match RateLimitRequests")
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout, "Expected IdleConnTimeout 90s")
}

func TestClient_Semaphore_Size(t *testing.T) {
	tempDir := t.TempDir()
	cacheManager, _ := cache.NewFileManager(tempDir)
	client := NewClient("123456", "test-token", cacheManager)

	expectedCapacity := RateLimitRequests
	actualCapacity := cap(client.semaphore)
	assert.Equal(t, expectedCapacity, actualCapacity, "Semaphore capacity should match RateLimitRequests")
}
