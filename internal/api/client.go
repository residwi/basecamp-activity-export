package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/residwi/bcreview/internal/cache"
	"github.com/residwi/bcreview/internal/ratelimit"
	"github.com/residwi/bcreview/internal/tracking"
)

const (
	BaseURL = "https://3.basecampapi.com"

	// UserAgent for API requests (https://github.com/basecamp/bc3-api/blob/master/README.md#identifying-your-application)
	UserAgent = "Basecamp Activity for Performance Review (resi.thawasa@vidio.com)"

	// Rate limiting settings
	RateLimitRequests = 50
	RateLimitWindow   = 10 * time.Second
)

type Client struct {
	httpClient    *http.Client
	rateLimiter   ratelimit.Limiter
	cache         cache.Manager
	semaphore     chan struct{}
	accountID     string
	accessToken   string
	baseURL       string
	userAgent     string
	rateLimitHits atomic.Uint64
}

// pageResult holds the result of fetching a single page
type pageResult struct {
	pageNum int
	items   []json.RawMessage
	err     error
}

// metaItem is a lightweight struct for date checking without loading full payload
type metaItem struct {
	CreatedAt time.Time `json:"created_at"`
}

// probeResult holds the result of probing a specific page
type probeResult struct {
	pageNum       int
	hasData       bool
	hasTargetYear bool
	err           error
}

func NewClient(accountID, accessToken string, cacheManager cache.Manager) *Client {
	// Optimize transport for high concurrency to the same host
	// Default transport only allows 2 idle conns per host, causing connection churn
	transport := &http.Transport{
		MaxIdleConns:        RateLimitRequests,
		MaxIdleConnsPerHost: RateLimitRequests,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		accountID:   accountID,
		accessToken: accessToken,
		baseURL:     BaseURL + "/" + accountID,
		userAgent:   UserAgent,
		rateLimiter: ratelimit.NewSlidingWindowLimiter(RateLimitRequests, RateLimitWindow),
		semaphore:   make(chan struct{}, RateLimitRequests),
		cache:       cacheManager,
	}
}

// GetRateLimitHits returns the number of times requests hit the rate limit (429 responses)
func (c *Client) GetRateLimitHits() uint64 {
	return c.rateLimitHits.Load()
}

// execute performs the HTTP request with rate limiting, caching, and tracking
func (c *Client) execute(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	fullURL := c.buildURL(path, query)

	var statusCode int
	if tracker, ok := tracking.TrackerFromContext(ctx); ok {
		if recType, ok := tracking.ResourceTypeFromContext(ctx); ok {
			tracker.StartRequest(recType, fullURL)
			defer func() {
				tracker.CompleteRequest(recType, fullURL, statusCode)
			}()
		}
	}

	var cached *cache.Entry
	if c.cache != nil {
		if cached, _ = c.cache.Get(ctx, fullURL); cached != nil {
			// Optimized: If cache allows us to skip the network completely (e.g. valid TTL),
			// we would return here. Currently logic requires 304 checks, so we proceed.
			// (Preserved original ETag logic for correctness with API behavior)
		}
	}

	// Retry Loop
	for {
		// 1. Rate Limit Wait (Performed OUTSIDE semaphore)
		for !c.rateLimiter.Allow() {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(25 * time.Millisecond):
				continue
			}
		}

		var retryWait time.Duration
		var resp *http.Response
		var err error

		resp, shouldRetry, err := func() (*http.Response, bool, error) {
			// Acquire Semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return nil, false, ctx.Err()
			}

			req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
			if err != nil {
				return nil, false, fmt.Errorf("creating request: %w", err)
			}

			req.Header.Set("Authorization", "Bearer "+c.accessToken)
			req.Header.Set("User-Agent", c.userAgent)
			req.Header.Set("Content-Type", "application/json")

			if cached != nil {
				if cached.ETag != "" {
					req.Header.Set("If-None-Match", cached.ETag)
				}
				if cached.LastModified != "" {
					req.Header.Set("If-Modified-Since", cached.LastModified)
				}
			}

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, false, fmt.Errorf("executing request: %w", err)
			}
			statusCode = resp.StatusCode

			// Handle 429 Retry
			if resp.StatusCode == http.StatusTooManyRequests {
				tracker, hasTracker := tracking.TrackerFromContext(ctx)
				recType, hasRecType := tracking.ResourceTypeFromContext(ctx)

				c.rateLimitHits.Add(1)
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				retryAfter := resp.Header.Get("Retry-After")
				if retryAfter != "" {
					if d, err := strconv.Atoi(retryAfter); err == nil {
						retryWait = time.Duration(d) * time.Second
						if hasTracker && hasRecType {
							tracker.RecordRateLimitWait(recType, retryWait, "429 Too Many Requests")
						}
					}
				}

				return nil, true, nil
			}

			// Handle 304 Not Modified
			if resp.StatusCode == http.StatusNotModified && cached != nil {
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(cached.Body))
				resp.StatusCode = http.StatusOK

				return resp, false, nil
			}

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, false, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
			}

			if c.cache != nil && resp.StatusCode == http.StatusOK {
				etag := resp.Header.Get("ETag")
				lastModified := resp.Header.Get("Last-Modified")

				if etag != "" || lastModified != "" {
					body, errReader := io.ReadAll(resp.Body)
					resp.Body.Close()
					if errReader != nil {
						return nil, false, fmt.Errorf("reading response: %w", errReader)
					}

					_ = c.cache.Set(ctx, fullURL, etag, lastModified, body)
					resp.Body = io.NopCloser(bytes.NewReader(body))
				}
			}

			return resp, false, nil
		}()

		if shouldRetry {
			// Track rate limit wait
			if tracker, ok := tracking.TrackerFromContext(ctx); ok {
				if recType, ok := tracking.ResourceTypeFromContext(ctx); ok {
					tracker.RecordRateLimitWait(recType, retryWait, "429 Too Many Requests")
				}
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryWait):
				continue
			}
		}

		return resp, err
	}
}

// get performs a GET request and decodes JSON response
func (c *Client) get(ctx context.Context, path string, query url.Values, result any) error {
	resp, err := c.execute(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}
	defer func() {
		// Drain body to allow TCP connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if result == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}

	return nil
}

func (c *Client) getPaginated(ctx context.Context, path string, query url.Values, year int) ([]json.RawMessage, error) {
	tracker, hasTracker := tracking.TrackerFromContext(ctx)
	recType, hasRecType := tracking.ResourceTypeFromContext(ctx)

	lastPage, err := c.findLastPage(ctx, path, query, year)
	if err != nil {
		if hasTracker && hasRecType {
			tracker.FailTrace(recType, err)
		}
		return nil, err
	}

	tracker.SetLastPageFound(recType, lastPage)

	if lastPage == 0 {
		return []json.RawMessage{}, nil
	}

	results, err := c.fetchPagesParallel(ctx, path, query, year, lastPage)
	if err != nil {
		if hasTracker && hasRecType {
			tracker.FailTrace(recType, err)
		}
		return nil, err
	}

	if hasTracker && hasRecType {
		tracker.SetTotalItems(recType, len(results))
	}

	return results, nil
}

// fetchPagesParallel fetches multiple pages concurrently using semaphore for rate limiting
func (c *Client) fetchPagesParallel(ctx context.Context, path string, query url.Values, year int, lastPage int) ([]json.RawMessage, error) {
	tracker, hasTracker := tracking.TrackerFromContext(ctx)
	recType, hasRecType := tracking.ResourceTypeFromContext(ctx)
	if hasTracker && hasRecType {
		tracker.StartSpan(recType, tracking.SpanFetchData)
		defer tracker.CompleteSpan(recType, tracking.SpanFetchData)
	}

	resultsCh := make(chan pageResult, lastPage)
	var wg sync.WaitGroup

	for pageNum := 1; pageNum <= lastPage; pageNum++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()

			pageQuery := maps.Clone(query)
			pageQuery.Set("page", fmt.Sprintf("%d", page))

			resp, err := c.execute(ctx, http.MethodGet, path, pageQuery)
			if err != nil {
				resultsCh <- pageResult{pageNum: page, err: err}
				return
			}

			defer func() {
				// Drain body to allow TCP connection reuse
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}()

			var pageItems []json.RawMessage
			if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
				resultsCh <- pageResult{pageNum: page, err: fmt.Errorf("decoding page %d: %w", page, err)}
				return
			}

			resultsCh <- pageResult{pageNum: page, items: pageItems}
		}(pageNum)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	pages := make(map[int][]json.RawMessage, lastPage)
	for res := range resultsCh {
		if res.err != nil {
			return nil, res.err
		}
		pages[res.pageNum] = res.items
	}

	var results []json.RawMessage

	for pageNum := 1; pageNum <= lastPage; pageNum++ {
		items := pages[pageNum]
		if len(items) == 0 {
			continue
		}

		// Check the LAST item (oldest) if it is inside the year, the WHOLE page is inside the year (because sorted DESC).
		var lastItemMeta metaItem
		if err := json.Unmarshal(items[len(items)-1], &lastItemMeta); err == nil {
			if lastItemMeta.CreatedAt.Year() >= year {
				// Fast path: Append entire page without individual unmarshaling
				results = append(results, items...)
				continue
			}
		}

		// Slow path: This is the boundary page (or mixed data). Filter item by item.
		for _, item := range items {
			var m metaItem
			if err := json.Unmarshal(item, &m); err == nil {
				if m.CreatedAt.Year() < year {
					// Stop immediately once we cross the year boundary
					return results, nil
				}
			}
			results = append(results, item)
		}
	}

	return results, nil
}

// findLastPage uses concurrent exponential probing to find the boundary page
// Returns the last page number that contains records from the target year
func (c *Client) findLastPage(ctx context.Context, path string, query url.Values, year int) (int, error) {
	tracker, hasTracker := tracking.TrackerFromContext(ctx)
	recType, hasRecType := tracking.ResourceTypeFromContext(ctx)
	if hasTracker && hasRecType {
		tracker.StartSpan(recType, tracking.SpanFindLastPage)
		defer tracker.CompleteSpan(recType, tracking.SpanFindLastPage)
	}

	// Exponential probing to find upper bound (probe pages 1, 2, 4, 8, 16, 32, 64, 128...)
	probePages := []int{1, 2, 4, 8, 16, 32, 64, 128, 256}

	probeCh := make(chan probeResult, len(probePages))
	cancelFuncs := make(map[int]context.CancelFunc, len(probePages))

	// Ensure all contexts are cleaned up when this function exits
	defer func() {
		for _, cancel := range cancelFuncs {
			cancel()
		}
	}()

	for _, page := range probePages {
		pageCtx, cancel := context.WithCancel(ctx)
		cancelFuncs[page] = cancel

		go func(p int) {
			probeCh <- c.probePage(pageCtx, path, query, year, p)
		}(page)
	}

	probes := make(map[int]probeResult)
	for range probePages {
		result := <-probeCh
		probes[result.pageNum] = result

		// kill switch: If a probe finds no data or no target year,
		// cancel all higher page probes to save bandwidth and connections.
		if result.err == nil && (!result.hasData || !result.hasTargetYear) {
			for p, cancel := range cancelFuncs {
				if p > result.pageNum {
					cancel()
				}
			}
		}
	}

	// lastGood: The highest page number we know has data from the target year
	// upperBound: The lowest page number we know does not have data (it's either empty or too old).
	lastGood, upperBound := 0, 0
	foundUpperBound := false

	for _, pageNum := range probePages {
		res := probes[pageNum]
		if res.err != nil || !res.hasData || !res.hasTargetYear {
			upperBound = pageNum
			foundUpperBound = true
			break
		}
		lastGood = pageNum
	}

	if !foundUpperBound {
		upperBound = probePages[len(probePages)-1] * 2 // Guess a higher bound
	}

	// Binary search between lastGood and upperBound to find the exact last relevant page
	for lastGood < upperBound-1 {
		mid := (lastGood + upperBound) / 2
		res := c.probePage(ctx, path, query, year, mid)
		if res.err != nil {
			return lastGood, nil
		}

		if res.hasData && res.hasTargetYear {
			lastGood = mid
		} else {
			upperBound = mid
		}
	}

	return lastGood, nil
}

// probePage checks if a specific page has data and contains target year records
func (c *Client) probePage(ctx context.Context, path string, query url.Values, year int, pageNum int) probeResult {
	pageQuery := url.Values{}
	if query != nil {
		maps.Copy(pageQuery, query)
	}
	pageQuery.Set("page", strconv.Itoa(pageNum))

	resp, err := c.execute(ctx, http.MethodGet, path, pageQuery)
	if err != nil {
		return probeResult{pageNum: pageNum, err: err}
	}
	defer func() {
		// Drain body to allow TCP connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	var page []metaItem
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return probeResult{pageNum: pageNum, err: err}
	}

	if len(page) == 0 {
		return probeResult{pageNum: pageNum, hasData: false}
	}

	hasTargetYear := page[len(page)-1].CreatedAt.Year() >= year

	return probeResult{
		pageNum:       pageNum,
		hasData:       true,
		hasTargetYear: hasTargetYear,
	}
}

// buildURL constructs the full URL for a request
func (c *Client) buildURL(path string, query url.Values) string {
	if strings.HasPrefix(path, c.baseURL) {
		return path
	}
	base := c.baseURL + path
	if len(query) == 0 {
		return base
	}

	return base + "?" + query.Encode()
}
