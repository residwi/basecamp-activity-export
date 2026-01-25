package basecamp

import (
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
	"time"

	"github.com/residwi/basecamp-activity-export/internal/trace"
)

const (
	maxErrorBodySize = 4 << 10 // 4KB

	// Basecamp geared pagination thresholds
	firstPageSize   = 15
	secondPageSize  = 30
	thirdPageSize   = 50
	defaultPageSize = 100 // standard Basecamp page size after page 3
)

// Client provides methods to interact with the Basecamp API
type Client struct {
	httpClient  *http.Client
	accountID   string
	accessToken string
	baseURL     string
	userAgent   string
	semaphore   chan struct{}
}

// pageResult holds the result of fetching a single page
type pageResult struct {
	pageNum int
	items   []json.RawMessage
	err     error
}

// metaItem is a lightweight struct for date extraction without full deserialization
type metaItem struct {
	CreatedAt time.Time `json:"created_at"`
}

func NewClient(options ...optionFunc) *Client {
	client := &Client{}
	for _, opt := range options {
		opt(client)
	}

	return client
}

// Get performs a GET request and decodes the JSON response
func (c *Client) Get(ctx context.Context, path string, query url.Values, result any) error {
	resp, err := c.execute(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}
	defer closeBody(resp.Body)

	if result == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// GetPaginated fetches all pages of a paginated resource, filtering by year
func (c *Client) GetPaginated(ctx context.Context, path string, query url.Values, year int) ([]json.RawMessage, error) {
	tc := newTrackingContext(ctx)

	firstPageItems, totalItems, err := c.fetchFirstPage(ctx, path, query)
	if err != nil {
		return nil, err
	}

	if len(firstPageItems) == 0 {
		return []json.RawMessage{}, nil
	}

	var lastItem metaItem
	if err := json.Unmarshal(firstPageItems[len(firstPageItems)-1], &lastItem); err == nil {
		if lastItem.CreatedAt.Year() < year {
			return filterItemsByYear(firstPageItems, year), nil
		}
	}

	maxPages := calculateTotalPages(totalItems)
	if maxPages <= 1 {
		return filterItemsByYear(firstPageItems, year), nil
	}

	lastPage, err := c.findLastPageByYear(ctx, path, query, year, maxPages)
	if err != nil {
		tc.fail(err)
		return nil, err
	}
	tc.setLastPageFound(lastPage)

	results, err := c.fetchPagesParallel(ctx, path, query, year, lastPage, firstPageItems)
	if err != nil {
		tc.fail(err)
		return nil, err
	}

	return results, nil
}

// fetchFirstPage retrieves the first page and total count from headers
func (c *Client) fetchFirstPage(ctx context.Context, path string, query url.Values) ([]json.RawMessage, int, error) {
	tc := newTrackingContext(ctx)

	pageQuery := withPage(query, 1)

	resp, err := c.execute(ctx, http.MethodGet, path, pageQuery)
	if err != nil {
		return nil, 0, err
	}
	defer closeBody(resp.Body)

	totalItems, _ := strconv.Atoi(resp.Header.Get("X-Total-Count"))

	var items []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, 0, fmt.Errorf("decoding first page: %w", err)
	}

	tc.setTotalItems(totalItems)

	return items, totalItems, nil
}

// findLastPageByYear uses bounded galloping followed by binary search to efficiently
// locate the last page containing records from the target year
func (c *Client) findLastPageByYear(ctx context.Context, path string, query url.Values, year, maxPages int) (int, error) {
	tc := newTrackingContext(ctx)
	tc.startSpan(trace.SpanFindLastPage)
	defer tc.completeSpan()

	lastGood, upperBound, err := c.gallopSearch(ctx, path, query, year, maxPages)
	if err != nil {
		return lastGood, err
	}

	if (upperBound - lastGood) <= 1 {
		return lastGood, nil
	}

	return c.binarySearch(ctx, path, query, year, lastGood, upperBound)
}

// gallopSearch performs exponential search to find the upper bound
func (c *Client) gallopSearch(ctx context.Context, path string, query url.Values, year, maxPages int) (lastGood, upperBound int, err error) {
	const growthFactor = 2
	lastGood, upperBound = 1, 1

	for upperBound < maxPages {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}

		nextPage := min(upperBound*growthFactor, maxPages)

		hasData, hasYear, err := c.probePage(ctx, path, query, year, nextPage)
		if err != nil {
			return lastGood, nextPage, err
		}

		if hasData && hasYear {
			lastGood = nextPage
			upperBound = nextPage
			if lastGood == maxPages {
				return maxPages, maxPages, nil
			}
		} else {
			return lastGood, nextPage, nil
		}
	}

	return lastGood, maxPages, nil
}

// binarySearch narrows down the exact last page within the bounded range
func (c *Client) binarySearch(ctx context.Context, path string, query url.Values, year, low, high int) (int, error) {
	for low < high-1 {
		if err := ctx.Err(); err != nil {
			return low, err
		}

		// overflow-safe midpoint calculation
		mid := low + (high-low)/2

		hasData, hasYear, err := c.probePage(ctx, path, query, year, mid)
		if err != nil {
			return low, err
		}

		if hasData && hasYear {
			low = mid
		} else {
			high = mid
		}
	}
	return low, nil
}

// fetchPagesParallel fetches multiple pages concurrently controlled by semaphore
func (c *Client) fetchPagesParallel(ctx context.Context, path string, query url.Values, year, lastPage int, firstPage []json.RawMessage) ([]json.RawMessage, error) {
	tc := newTrackingContext(ctx)
	tc.startSpan(trace.SpanFetchData)
	defer tc.completeSpan()

	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pagesToFetch := lastPage - 1
	resultsCh := make(chan pageResult, pagesToFetch)

	var wg sync.WaitGroup
	wg.Add(pagesToFetch)

	for pageNum := 2; pageNum <= lastPage; pageNum++ {
		go c.fetchPageWorker(fetchCtx, &wg, resultsCh, path, query, pageNum)
	}

	go func() {
		defer close(resultsCh)
		wg.Wait()
	}()

	return collectResults(ctx, resultsCh, firstPage, lastPage, year)
}

// fetchPageWorker fetches a single page with semaphore-controlled concurrency
func (c *Client) fetchPageWorker(ctx context.Context, wg *sync.WaitGroup, results chan<- pageResult, path string, query url.Values, pageNum int) {
	defer wg.Done()

	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		results <- pageResult{pageNum: pageNum, err: ctx.Err()}
		return
	}

	pageQuery := withPage(query, pageNum)
	items := make([]json.RawMessage, 0, defaultPageSize)
	err := c.Get(ctx, path, pageQuery, &items)

	results <- pageResult{pageNum: pageNum, items: items, err: err}
}

// probePage checks if a page has data containing the target year.
func (c *Client) probePage(ctx context.Context, path string, query url.Values, year, pageNum int) (hasData, hasTargetYear bool, err error) {
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return false, false, ctx.Err()
	}

	var page []metaItem
	pageQuery := withPage(query, pageNum)
	err = c.Get(ctx, path, pageQuery, &page)
	if err != nil {
		return false, false, fmt.Errorf("decoding probe page %d: %w", pageNum, err)
	}

	if len(page) == 0 {
		return false, false, nil
	}

	lastItem := page[len(page)-1]
	hasTargetYear = lastItem.CreatedAt.Year() >= year

	return true, hasTargetYear, nil
}

// execute performs an HTTP request with proper error handling.
func (c *Client) execute(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path, query), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer closeBody(resp.Body)
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, body)
	}

	return resp, nil
}

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

// collectResults gathers results from the channel and assembles them in order
func collectResults(ctx context.Context, resultsCh <-chan pageResult, firstPage []json.RawMessage, lastPage, year int) ([]json.RawMessage, error) {
	pages := make(map[int][]json.RawMessage, lastPage)
	pages[1] = firstPage

	var firstErr error
	for res := range resultsCh {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		pages[res.pageNum] = res.items
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}

	return assembleFilteredResults(pages, lastPage, year), nil
}

// assembleFilteredResults combines pages in order with year filtering.
func assembleFilteredResults(pages map[int][]json.RawMessage, lastPage, year int) []json.RawMessage {
	estimatedItemsPerPage := 50 // rough average for preallocation
	results := make([]json.RawMessage, 0, lastPage*estimatedItemsPerPage)

	for pageNum := 1; pageNum <= lastPage; pageNum++ {
		items, ok := pages[pageNum]
		if !ok {
			break
		}

		filtered := filterItemsByYear(items, year)
		results = append(results, filtered...)

		if len(filtered) < len(items) {
			break
		}
	}

	return results
}

// closeBody drains and closes the response body to enable connection reuse.
func closeBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

func withPage(query url.Values, page int) url.Values {
	result := make(url.Values, len(query)+1)
	maps.Copy(result, query)
	result.Set("page", strconv.Itoa(page))
	return result
}

// calculateTotalPages computes page count for Basecamp's geared pagination.
// Page sizes: 15, 30, 50, 100, 100, ...
func calculateTotalPages(totalItems int) int {
	if totalItems <= 0 {
		return 0
	}

	thresholds := [3]int{
		firstPageSize,                                  // 15
		firstPageSize + secondPageSize,                 // 45
		firstPageSize + secondPageSize + thirdPageSize, // 95
	}

	for i, threshold := range thresholds {
		if totalItems <= threshold {
			return i + 1
		}
	}

	remaining := totalItems - thresholds[2]
	return 3 + (remaining+defaultPageSize-1)/defaultPageSize
}

// filterItemsByYear returns items created in or after the specified year
func filterItemsByYear(items []json.RawMessage, year int) []json.RawMessage {
	result := make([]json.RawMessage, 0, len(items))

	for _, item := range items {
		var m metaItem
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}

		if m.CreatedAt.Year() < year {
			break
		}
		result = append(result, item)
	}

	return result
}
