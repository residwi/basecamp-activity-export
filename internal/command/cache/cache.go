package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/config"
	"github.com/residwi/basecamp-activity-export/internal/httpcache"
)

type Statistics struct {
	TotalFiles         int64
	TotalSizeFormatted string
	CacheDir           string
	OldestEntry        *time.Time
	NewestEntry        *time.Time
}

// Clear removes all cache entries
func Clear(ctx context.Context) error {
	cacheDir := config.CacheDir()
	cacheManager, err := httpcache.NewFileManager(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	if err := cacheManager.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

// Stats returns cache statistics
func Stats(ctx context.Context) (Statistics, error) {
	cacheDir := config.CacheDir()
	cacheManager, err := httpcache.NewFileManager(cacheDir)
	if err != nil {
		return Statistics{}, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	cacheStats, err := cacheManager.Stats(ctx)
	if err != nil {
		return Statistics{}, fmt.Errorf("failed to get cache stats: %w", err)
	}

	stats := Statistics{
		TotalFiles:         cacheStats.TotalEntries,
		TotalSizeFormatted: formatSize(cacheStats.TotalSize),
		CacheDir:           cacheDir,
		OldestEntry:        cacheStats.OldestEntry,
		NewestEntry:        cacheStats.NewestEntry,
	}

	return stats, nil
}

// formatSize converts bytes to human-readable format
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
