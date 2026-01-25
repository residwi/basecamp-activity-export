package httpcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileManager(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tempDir := t.TempDir()

		fm, err := NewFileManager(tempDir)
		require.NoError(t, err, "NewFileManager should not fail")

		assert.Equal(t, tempDir, fm.cacheDir, "Cache directory should match")
		assert.Equal(t, time.Duration(0), fm.ttl, "TTL should be 0")

		_, err = os.Stat(tempDir)
		assert.False(t, os.IsNotExist(err), "Cache directory should exist")
	})

	t.Run("with ttl", func(t *testing.T) {
		tempDir := t.TempDir()
		ttl := 1 * time.Hour

		fm, err := NewFileManagerWithTTL(tempDir, ttl)
		require.NoError(t, err, "NewFileManagerWithTTL should not fail")

		assert.Equal(t, tempDir, fm.cacheDir, "Cache directory should match")
		assert.Equal(t, ttl, fm.ttl, "TTL should match")

		_, err = os.Stat(tempDir)
		assert.False(t, os.IsNotExist(err), "Cache directory should exist")
	})

	t.Run("error create directory", func(t *testing.T) {
		tempDir := t.TempDir()
		readOnlyDir := filepath.Join(tempDir, "readonly")

		err := os.Mkdir(readOnlyDir, 0o400)
		require.NoError(t, err, "Failed to create read-only dir")

		invalidCache := filepath.Join(readOnlyDir, "cache")
		_, err = NewFileManager(invalidCache)
		require.Error(t, err, "Expected error creating FileManager in read-only dir")
	})
}

func TestFileManager_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fm := setupTestFileManager(t)

		url := "https://api.example.com/test"
		etag := `"abc123"`
		lastModified := "Mon, 01 Jan 2025 00:00:00 GMT"
		body := []byte(`{"data": "test"}`)

		err := fm.Set(t.Context(), url, etag, lastModified, body)
		require.NoError(t, err, "Set failed")

		entry, err := fm.Get(t.Context(), url)
		require.NoError(t, err, "Get failed")

		assert.Equal(t, url, entry.URL, "URL should match")
		assert.Equal(t, etag, entry.ETag, "ETag should match")
		assert.Equal(t, lastModified, entry.LastModified, "LastModified should match")

		var expectedJSON, actualJSON map[string]any
		json.Unmarshal(body, &expectedJSON)
		json.Unmarshal(entry.Body, &actualJSON)

		assert.Equal(t, len(expectedJSON), len(actualJSON), "Body JSON length should match")
		assert.WithinDuration(t, time.Now(), entry.CachedAt, time.Minute, "CachedAt should be recent")
	})

	t.Run("not found", func(t *testing.T) {
		fm := setupTestFileManager(t)

		_, err := fm.Get(t.Context(), "https://api.example.com/nonexistent")

		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("corrupted cache", func(t *testing.T) {
		fm := setupTestFileManager(t)

		url := "https://api.example.com/test"

		filename := fm.cacheFilename(url)
		corruptedData := []byte(`{invalid json}`)

		err := os.WriteFile(filename, corruptedData, 0o600)
		require.NoError(t, err, "Failed to write corrupted cache")

		_, err = fm.Get(t.Context(), url)
		assert.Error(t, err, "Expected error for corrupted cache")

		_, err = os.Stat(filename)
		assert.True(t, os.IsNotExist(err), "Corrupted cache file should have been deleted")
	})

	t.Run("expired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tempDir := t.TempDir()
			fm, _ := NewFileManagerWithTTL(tempDir, 10*time.Millisecond)

			url := "https://api.example.com/test"
			body := []byte(`{"data": "test"}`)

			err := fm.Set(t.Context(), url, "", "", body)
			require.NoError(t, err, "Set failed")

			entry, err := fm.Get(t.Context(), url)
			require.NoError(t, err, "Get failed before TTL")
			require.NotNil(t, entry, "Expected entry before TTL")

			time.Sleep(15 * time.Millisecond)

			_, err = fm.Get(t.Context(), url)
			assert.True(t, os.IsNotExist(err), "Expected os.ErrNotExist after TTL, got %v", err)
		})
	})
}

func TestFileManager_Set(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fm := setupTestFileManager(t)

		url := "https://api.example.com/test"
		etag := `"abc123"`
		lastModified := "Mon, 01 Jan 2025 00:00:00 GMT"
		body := []byte(`{"data": "test"}`)

		err := fm.Set(t.Context(), url, etag, lastModified, body)
		require.NoError(t, err, "Set failed")

		entry, err := fm.Get(t.Context(), url)
		require.NoError(t, err, "Get failed")

		assert.Equal(t, url, entry.URL, "URL should match")
		assert.Equal(t, etag, entry.ETag, "ETag should match")
		assert.Equal(t, lastModified, entry.LastModified, "LastModified should match")
		assert.WithinDuration(t, time.Now(), entry.CachedAt, time.Minute, "CachedAt should be recent")

		var expectedJSON, actualJSON map[string]any
		json.Unmarshal(body, &expectedJSON)
		json.Unmarshal(entry.Body, &actualJSON)
		assert.Equal(t, expectedJSON, actualJSON, "Body JSON should match")
	})

	t.Run("atomic write", func(t *testing.T) {
		fm := setupTestFileManager(t)

		url := "https://api.example.com/test"
		body := []byte(`{"data": "test"}`)

		err := fm.Set(t.Context(), url, "", "", body)
		require.NoError(t, err, "Set failed")

		tempFile := fm.cacheFilename(url) + ".tmp"
		_, err = os.Stat(tempFile)
		assert.ErrorIs(t, err, os.ErrNotExist)

		cacheFile := fm.cacheFilename(url)
		_, err = os.Stat(cacheFile)
		require.NoError(t, err, "Cache file should exist")
	})

	t.Run("error JSON marshal", func(t *testing.T) {
		fm := setupTestFileManager(t)

		url := "https://api.example.com/test"

		// intentionally passing an invalid body type to cause JSON marshal error
		err := fm.Set(t.Context(), url, "", "", []byte{0xff, 0xfe, 0xfd}) // invalid UTF-8
		require.Error(t, err, "Expected error for invalid body type")

		_, err = fm.Get(t.Context(), url)
		assert.ErrorIs(t, err, os.ErrNotExist, "Cache file should not exist after failed Set")
	})

	t.Run("overwrite existing", func(t *testing.T) {
		fm := setupTestFileManager(t)

		url := "https://api.example.com/test"

		body1 := []byte(`{"version": 1}`)
		err := fm.Set(t.Context(), url, "etag1", "", body1)
		require.NoError(t, err, "First Set failed")

		body2 := []byte(`{"version": 2}`)
		err = fm.Set(t.Context(), url, "etag2", "", body2)
		require.NoError(t, err, "Second Set failed")

		entry, err := fm.Get(t.Context(), url)
		require.NoError(t, err, "Get failed")
		assert.Equal(t, "etag2", entry.ETag, "Expected updated ETag etag2")

		var expectedJSON, actualJSON map[string]any
		json.Unmarshal(body2, &expectedJSON)
		json.Unmarshal(entry.Body, &actualJSON)
		assert.Equal(t, expectedJSON, actualJSON, "Expected updated body")
	})

	t.Run("error creating temp file", func(t *testing.T) {
		fm := setupTestFileManager(t)

		fm.cacheDir = "/nonexistent/directory/path"

		url := "https://api.example.com/test"
		body := []byte(`{"data": "test"}`)

		err := fm.Set(t.Context(), url, "", "", body)
		require.Error(t, err, "Expected error creating temp file")
	})
}

func TestFileManager_Clear(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fm := setupTestFileManager(t)

		// add dir to be ignored
		ignoredDir := filepath.Join(fm.cacheDir, "ignored_subdir")
		_ = os.Mkdir(ignoredDir, 0o700)

		urls := []string{
			"https://api.example.com/test1",
			"https://api.example.com/test2",
			"https://api.example.com/test3",
		}

		for _, url := range urls {
			err := fm.Set(t.Context(), url, "", "", []byte(`{}`))
			require.NoError(t, err, "Set failed for %s", url)
		}

		err := fm.Clear(t.Context())
		require.NoError(t, err, "Clear failed")

		for _, url := range urls {
			_, err := fm.Get(t.Context(), url)
			assert.ErrorIs(t, err, os.ErrNotExist)
		}
	})

	t.Run("empty cache", func(t *testing.T) {
		fm := setupTestFileManager(t)

		err := fm.Clear(t.Context())
		assert.NoError(t, err, "Clear empty cache failed")
	})

	t.Run("error read non-exist directory", func(t *testing.T) {
		fm := setupTestFileManager(t)

		fm.cacheDir = "/nonexistent/directory/path"

		err := fm.Clear(t.Context())
		require.Error(t, err, "Expected error clearing non-existent directory")
	})
}

func TestFileManager_Stats(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fm := setupTestFileManager(t)

		// add dir to be ignored
		ignoredDir := filepath.Join(fm.cacheDir, "ignored_subdir")
		_ = os.Mkdir(ignoredDir, 0o700)

		urls := []string{
			"https://api.example.com/test1",
			"https://api.example.com/test2",
			"https://api.example.com/test3",
		}

		for _, url := range urls {
			body := []byte(`{"url": "` + url + `"}`)
			err := fm.Set(t.Context(), url, "", "", body)
			require.NoError(t, err, "Set failed")
		}

		stats, err := fm.Stats(t.Context())
		require.NoError(t, err, "Stats failed")

		assert.Equal(t, int64(3), stats.TotalEntries, "Expected 3 entries")
		assert.NotNil(t, stats.OldestEntry, "Expected OldestEntry to be set")
		assert.NotNil(t, stats.NewestEntry, "Expected NewestEntry to be set")

		if stats.OldestEntry != nil && stats.NewestEntry != nil {
			assert.False(t, stats.NewestEntry.Before(*stats.OldestEntry), "NewestEntry should not be before OldestEntry")
		}
	})

	t.Run("empty cache", func(t *testing.T) {
		fm := setupTestFileManager(t)

		stats, err := fm.Stats(t.Context())
		require.NoError(t, err, "Stats failed")

		assert.Equal(t, int64(0), stats.TotalEntries, "Expected 0 entries")
		assert.Equal(t, int64(0), stats.TotalSize, "Expected 0 size")
		assert.Nil(t, stats.OldestEntry, "Expected nil OldestEntry for empty cache")
		assert.Nil(t, stats.NewestEntry, "Expected nil NewestEntry for empty cache")
	})

	t.Run("error read non-exist directory", func(t *testing.T) {
		fm := setupTestFileManager(t)

		fm.cacheDir = "/nonexistent/directory/path"

		_, err := fm.Stats(t.Context())
		require.Error(t, err, "Expected error getting stats for non-existent directory")
	})
}

func TestHashURL(t *testing.T) {
	t.Run("Deterministic", func(t *testing.T) {
		url := "https://api.example.com/test"

		hash1 := hashURL(url)
		hash2 := hashURL(url)

		assert.Equal(t, hash1, hash2, "Hash should be deterministic")

		assert.Equal(t, 64, len(hash1), "Expected 64-character hash")
	})

	t.Run("Different", func(t *testing.T) {
		url1 := "https://api.example.com/test1"
		url2 := "https://api.example.com/test2"

		hash1 := hashURL(url1)
		hash2 := hashURL(url2)

		assert.NotEqual(t, hash1, hash2, "Different URLs should have different hashes")
	})
}

func TestFileManager_ConcurrentAccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fm := setupTestFileManager(t)

		body := []byte(`{"data": "test"}`)

		const goroutines = 10
		const iterations = 50

		for i := range goroutines {
			go func() {
				for j := range iterations {
					url := fmt.Sprintf("https://api.example.com/test-%d-%d", i, j)
					err := fm.Set(t.Context(), url, "", "", body)
					require.NoError(t, err, "Set failed in goroutine")

					entry, err := fm.Get(t.Context(), url)
					require.NoError(t, err, "Get failed in goroutine")

					assert.Equal(t, url, entry.URL, "URL should match in goroutine")
				}
			}()
		}

		synctest.Wait()
	})
}

func setupTestFileManager(t *testing.T) *FileManager {
	t.Helper()
	tempDir := t.TempDir()
	fm, err := NewFileManager(tempDir)
	require.NoError(t, err, "Failed to create test FileManager")
	return fm
}
