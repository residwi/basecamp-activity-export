package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheClear(t *testing.T) {
	t.Run("executes clear successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		cacheDir := filepath.Join(tmpDir, ".bclog", "cache")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		testFile := filepath.Join(cacheDir, "test-cache.json")
		err = os.WriteFile(testFile, []byte("test data"), 0o644)
		require.NoError(t, err)

		err = Clear(t.Context())
		require.NoError(t, err, "clear command should execute successfully")

		_, err = os.Stat(testFile)
		assert.True(t, os.IsNotExist(err), "cache file should be removed")
	})

	t.Run("error on invalid cache directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "invalid-path")
		err := os.MkdirAll(cacheDir, 0o400)
		t.Setenv("HOME", cacheDir)

		err = Clear(t.Context())
		require.Error(t, err, "clear command should return error for invalid cache directory")
	})
}

func TestCacheStats(t *testing.T) {
	t.Run("executes stats with cache data", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		cacheDir := filepath.Join(tmpDir, ".bclog", "cache")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		urls := []string{
			"https://api.example.com/test1",
			"https://api.example.com/test2",
			"https://api.example.com/test3",
		}

		for _, url := range urls {
			body := []byte(`{"url": "` + url + `"}`)
			err = os.WriteFile(filepath.Join(cacheDir, filepath.Base(url)+".json"), body, 0o644)
			require.NoError(t, err, "Failed to set cache entry")
		}

		stats, err := Stats(t.Context())

		assert.Equal(t, int64(3), stats.TotalFiles, "Expected 3 files")
		assert.NotEmpty(t, stats.CacheDir, "CacheDir should be set")
		assert.NotNil(t, stats.OldestEntry, "OldestEntry should not be nil")
		assert.NotNil(t, stats.NewestEntry, "NewestEntry should not be nil")
	})

	t.Run("multiple cache entries size calculation", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		cacheDir := filepath.Join(tmpDir, ".bclog", "cache")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		for i := range 10 {
			url := "https://api.example.com/item" + string(rune('0'+i))
			body := []byte(`{"id": ` + string(rune('0'+i)) + `, "data": "test data for cache entry"}`)
			err := os.WriteFile(filepath.Join(cacheDir, filepath.Base(url)+".json"), body, 0o644)
			require.NoError(t, err, "Failed to set cache entry")
		}

		stats, err := Stats(t.Context())
		require.NoError(t, err, "GetStats should not fail")

		assert.Equal(t, int64(10), stats.TotalFiles, "Expected 10 files")
	})

	t.Run("executes stats with empty cache", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		cacheDir := filepath.Join(tmpDir, ".bclog", "cache")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		stats, err := Stats(t.Context())
		require.NoError(t, err, "stats command should execute successfully with empty cache")

		assert.Equal(t, int64(0), stats.TotalFiles, "Expected 0 files in empty cache")
		assert.NotEmpty(t, stats.CacheDir, "CacheDir should be set")
		assert.Nil(t, stats.OldestEntry, "OldestEntry should be nil for empty cache")
		assert.Nil(t, stats.NewestEntry, "NewestEntry should be nil for empty cache")
	})

	t.Run("error on invalid cache directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "invalid-path")
		err := os.MkdirAll(cacheDir, 0o400)
		t.Setenv("HOME", cacheDir)

		_, err = Stats(t.Context())
		require.Error(t, err, "stats command should return error for invalid cache directory")
	})
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 bytes",
		},
		{
			name:     "bytes",
			bytes:    500,
			expected: "500 bytes",
		},
		{
			name:     "kilobytes exactly 1KB",
			bytes:    1024,
			expected: "1.00 KB",
		},
		{
			name:     "kilobytes 1.5KB",
			bytes:    1536,
			expected: "1.50 KB",
		},
		{
			name:     "megabytes exactly 1MB",
			bytes:    1024 * 1024,
			expected: "1.00 MB",
		},
		{
			name:     "megabytes 2.5MB",
			bytes:    2621440, // 2.5 MB
			expected: "2.50 MB",
		},
		{
			name:     "gigabytes exactly 1GB",
			bytes:    1024 * 1024 * 1024,
			expected: "1.00 GB",
		},
		{
			name:     "gigabytes 3.25GB",
			bytes:    3489660928, // 3.25 GB
			expected: "3.25 GB",
		},
		{
			name:     "large gigabytes",
			bytes:    10 * 1024 * 1024 * 1024, // 10 GB
			expected: "10.00 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			assert.Equal(t, tt.expected, result, "Formatted size should match expected")
		})
	}
}

func TestFormatSize_EdgeCases(t *testing.T) {
	t.Run("boundary at 1023 bytes", func(t *testing.T) {
		result := formatSize(1023)
		assert.Equal(t, "1023 bytes", result)
	})

	t.Run("boundary at 1024 bytes becomes KB", func(t *testing.T) {
		result := formatSize(1024)
		assert.Equal(t, "1.00 KB", result)
	})

	t.Run("boundary at 1023 KB", func(t *testing.T) {
		result := formatSize(1023 * 1024)
		require.Contains(t, result, "KB")
	})

	t.Run("boundary at 1024 KB becomes MB", func(t *testing.T) {
		result := formatSize(1024 * 1024)
		assert.Equal(t, "1.00 MB", result)
	})

	t.Run("boundary at 1023 MB", func(t *testing.T) {
		result := formatSize(1023 * 1024 * 1024)
		require.Contains(t, result, "MB")
	})

	t.Run("boundary at 1024 MB becomes GB", func(t *testing.T) {
		result := formatSize(1024 * 1024 * 1024)
		assert.Equal(t, "1.00 GB", result)
	})
}
