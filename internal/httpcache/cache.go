package httpcache

import (
	"context"
	"encoding/json"
	"time"
)

// Entry represents a cached API response with freshness headers
type Entry struct {
	URL          string          `json:"url"`
	ETag         string          `json:"etag,omitempty"`
	LastModified string          `json:"last_modified,omitempty"`
	CachedAt     time.Time       `json:"cached_at"`
	Body         json.RawMessage `json:"body"`
}

// Stats provides cache statistics for monitoring and management
type Stats struct {
	TotalEntries int64      `json:"total_entries"`
	TotalSize    int64      `json:"total_size_bytes"`
	OldestEntry  *time.Time `json:"oldest_entry,omitempty"`
	NewestEntry  *time.Time `json:"newest_entry,omitempty"`
}

type Manager interface {
	Get(ctx context.Context, url string) (*Entry, error)
	Set(ctx context.Context, url string, etag, lastModified string, body []byte) error
	Clear(ctx context.Context) error
	Stats(ctx context.Context) (*Stats, error)
}
