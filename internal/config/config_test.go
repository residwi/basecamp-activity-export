package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustLoad(t *testing.T) {
	t.Run("loads config", func(t *testing.T) {
		// set all required environment variables
		t.Setenv("BASECAMP_ACCOUNT_ID", "1234567")
		t.Setenv("BASECAMP_CLIENT_ID", "test-client-id")
		t.Setenv("BASECAMP_CLIENT_SECRET", "test-client-secret")

		cfg := MustLoad()

		assert.Equal(t, "1234567", cfg.Basecamp.AccountID)
		assert.Equal(t, "test-client-id", cfg.Basecamp.ClientID)
		assert.Equal(t, "test-client-secret", cfg.Basecamp.ClientSecret)

		// check that optional fields have default values
		assert.Equal(t, "http://localhost:27072/callback", cfg.Basecamp.RedirectURI)
		assert.Equal(t, 300, cfg.Basecamp.RateLimit)
	})
}

func TestAppDir(t *testing.T) {
	t.Run("creates directory if not exists", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		expectedDir := filepath.Join(tempHome, ".bcreview")

		_, err := os.Stat(expectedDir)
		assert.ErrorIs(t, err, os.ErrNotExist)

		appDir := AppDir()
		assert.Equal(t, expectedDir, appDir)

		info, err := os.Stat(appDir)
		require.NoError(t, err, "directory should exist after calling AppDir")
		assert.True(t, info.IsDir(), "path should be a directory")

		mode := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0o700), mode, "directory should have 0700 permissions")
	})

	t.Run("returns existing directory path", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		expectedDir := filepath.Join(tempHome, ".bcreview")

		err := os.MkdirAll(expectedDir, 0o700)
		require.NoError(t, err)

		appDir := AppDir()
		assert.Equal(t, expectedDir, appDir)
	})
}

func TestTokenFile(t *testing.T) {
	t.Run("returns token json in app dir", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		expectedPath := filepath.Join(tempHome, ".bcreview", "token.json")

		tokenFile := TokenFile()

		assert.Equal(t, expectedPath, tokenFile)
	})
}

func TestCacheDir(t *testing.T) {
	t.Run("returns cache subdir in app dir", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		expectedPath := filepath.Join(tempHome, ".bcreview", "cache")

		cacheDir := CacheDir()

		assert.Equal(t, expectedPath, cacheDir)
	})
}
