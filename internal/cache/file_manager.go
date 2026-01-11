package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileManager struct {
	cacheDir string
	ttl      time.Duration
}

func NewFileManager(cacheDir string) (*FileManager, error) {
	return NewFileManagerWithTTL(cacheDir, 0)
}

func NewFileManagerWithTTL(cacheDir string, ttl time.Duration) (*FileManager, error) {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &FileManager{
		cacheDir: cacheDir,
		ttl:      ttl,
	}, nil
}

// Get retrieves a cached entry by URL
func (fm *FileManager) Get(ctx context.Context, url string) (*Entry, error) {
	filename := fm.cacheFilename(url)

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = os.Remove(filename)
		return nil, fmt.Errorf("corrupt cache entry deleted: %w", err)
	}

	if fm.ttl > 0 && time.Since(entry.CachedAt) > fm.ttl {
		return nil, os.ErrNotExist // expired cache treat as not found
	}

	return &entry, nil
}

// Set stores a cache entry with freshness headers
func (fm *FileManager) Set(ctx context.Context, url string, etag, lastModified string, body []byte) error {
	entry := Entry{
		URL:          url,
		ETag:         etag,
		LastModified: lastModified,
		CachedAt:     time.Now(),
		Body:         json.RawMessage(body),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	filename := fm.cacheFilename(url)

	// atomic write: write to temp file then rename
	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	_ = os.Rename(tempFile, filename)
	defer os.Remove(tempFile) // cleanup temp file if rename fails

	return nil
}

// Clear removes all cached entries
func (fm *FileManager) Clear(ctx context.Context) error {
	entries, err := os.ReadDir(fm.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := filepath.Join(fm.cacheDir, entry.Name())
		_ = os.Remove(filename)
	}

	return nil
}

func (fm *FileManager) Stats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		TotalEntries: 0,
		TotalSize:    0,
	}

	entries, err := os.ReadDir(fm.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		stats.TotalEntries++
		stats.TotalSize += info.Size()

		modTime := info.ModTime()
		if stats.OldestEntry == nil || modTime.Before(*stats.OldestEntry) {
			stats.OldestEntry = &modTime
		}
		if stats.NewestEntry == nil || modTime.After(*stats.NewestEntry) {
			stats.NewestEntry = &modTime
		}
	}

	return stats, nil
}

// cacheFilename generates a deterministic filename from URL using SHA256
func (fm *FileManager) cacheFilename(url string) string {
	hash := hashURL(url)
	return filepath.Join(fm.cacheDir, hash+".json")
}

func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])
}
