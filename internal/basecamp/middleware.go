package basecamp

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/httpcache"
	"github.com/residwi/basecamp-activity-export/internal/trace"
	"golang.org/x/time/rate"
)

type (
	Middleware       func(http.RoundTripper) http.RoundTripper
	RoundTripperFunc func(*http.Request) (*http.Response, error)
)

func (f RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func ChainTransport(base http.RoundTripper, middlewares ...Middleware) http.RoundTripper {
	transport := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		transport = middlewares[i](transport)
	}
	return transport
}

func Tracking() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			ctx := req.Context()
			tracker, hasTracker := trace.TrackerFromContext(ctx)
			recType, hasRecType := trace.ResourceTypeFromContext(ctx)

			if !hasTracker || !hasRecType {
				return next.RoundTrip(req)
			}

			var statusCode int
			fullURL := req.URL.String()

			tracker.StartRequest(recType, fullURL)

			// we use a closure to capture the 'statusCode' variable by reference
			// this ensures we get the updated value after RoundTrip finishes
			defer func() {
				tracker.CompleteRequest(recType, fullURL, statusCode)
			}()

			resp, err := next.RoundTrip(req)

			if resp != nil {
				statusCode = resp.StatusCode
			}

			return resp, err
		})
	}
}

func Cache(cacheManager httpcache.Manager) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				return next.RoundTrip(req)
			}

			fullURL := req.URL.String()
			cached, _ := cacheManager.Get(req.Context(), fullURL)

			if cached != nil {
				if cached.ETag != "" {
					req.Header.Set("If-None-Match", cached.ETag)
				}
				if cached.LastModified != "" {
					req.Header.Set("If-Modified-Since", cached.LastModified)
				}
			}

			resp, err := next.RoundTrip(req)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode == http.StatusNotModified && cached != nil {
				resp.Body.Close()
				// we set body first to avoid losing it in case of error from other middlewares
				resp.Body = io.NopCloser(bytes.NewReader(cached.Body))

				return resp, nil
			}

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				etag := resp.Header.Get("ETag")
				lastModified := resp.Header.Get("Last-Modified")

				if etag != "" || lastModified != "" {
					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					if err != nil {
						return nil, fmt.Errorf("reading response: %w", err)
					}

					_ = cacheManager.Set(req.Context(), fullURL, etag, lastModified, body)
					resp.Body = io.NopCloser(bytes.NewReader(body))
				}
			}

			return resp, nil
		})
	}
}

func Retry(maxRetries int) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var resp *http.Response
			var err error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				resp, err = next.RoundTrip(req)

				if err == nil && !isRetriable(resp.StatusCode, req.Method) {
					return resp, nil
				}

				if attempt == maxRetries {
					return resp, err
				}

				if resp != nil {
					resp.Body.Close()
				}

				wait := time.Duration(0)
				if resp != nil {
					wait = parseRetryAfter(resp.Header.Get("Retry-After"))

					tracker, hasTracker := trace.TrackerFromContext(req.Context())
					recType, hasRecType := trace.ResourceTypeFromContext(req.Context())
					if hasTracker && hasRecType {
						tracker.RecordRateLimitWait(recType, wait, http.StatusText(resp.StatusCode))
					}
				}

				if wait == 0 {
					// default Exponential Backoff for 5xx errors
					// 2^attempt * 500ms -> 500ms, 1s, 2s, 4s...
					base := float64(500 * time.Millisecond)
					factor := math.Pow(2, float64(attempt))
					// jitter: +/- 20% randomization to prevent thundering herd
					jitter := 0.8 + (rand.Float64() * 0.4)

					calculated := base * factor * jitter
					wait = time.Duration(calculated)

					cap := 30 * time.Second // don't wait longer than 30s
					if wait > cap {
						wait = cap
					}
				}

				select {
				case <-req.Context().Done():
					return nil, req.Context().Err()
				case <-time.After(wait):
				}

				// we cannot just use req.Body again because it has been read.
				// we must get a fresh reader.
				if req.GetBody != nil {
					newBody, err := req.GetBody()
					if err != nil {
						return nil, err
					}
					req.Body = newBody
				} else if req.Body != nil && req.Body != http.NoBody {
					// if Body exists but GetBody is nil, it's a streaming request.
					// we cannot retry streaming requests safely.
					return resp, err
				}
			}
			return resp, err
		})
	}
}

// isRetriable determines if a request should be retried based on status code and method.
func isRetriable(status int, method string) bool {
	if status == http.StatusTooManyRequests {
		return true
	}

	if status == http.StatusInternalServerError ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout {

		if method == http.MethodGet ||
			method == http.MethodHead ||
			method == http.MethodPut ||
			method == http.MethodDelete ||
			method == http.MethodOptions {
			return true
		}

		return false
	}

	return false
}

func parseRetryAfter(headerVal string) time.Duration {
	if headerVal == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(headerVal); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if dateTime, err := http.ParseTime(headerVal); err == nil {
		duration := time.Until(dateTime)
		if duration < 0 {
			return 0
		}
		return duration
	}
	return 0
}

func RateLimit(limiter *rate.Limiter) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if err := limiter.Wait(req.Context()); err != nil {
				return nil, err
			}
			return next.RoundTrip(req)
		})
	}
}
