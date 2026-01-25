package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenStore_Save(t *testing.T) {
	t.Run("saves token with correct expiration", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		token := TokenResponse{
			AccessToken:  "access-token-12345",
			RefreshToken: "refresh-token-67890",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}

		before := time.Now()
		err := s.save(token)
		require.NoError(t, err, "save should succeed")

		_, err = os.Stat(tokenPath)
		require.NoError(t, err, "token file should exist")

		info, err := os.Stat(tokenPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "file should have 0600 permissions")

		data, err := os.ReadFile(tokenPath)
		require.NoError(t, err)

		var stored storedToken
		err = json.Unmarshal(data, &stored)
		require.NoError(t, err, "stored token should be valid JSON")

		assert.Equal(t, token.AccessToken, stored.AccessToken, "access token should match")
		assert.Equal(t, token.RefreshToken, stored.RefreshToken, "refresh token should match")
		assert.Equal(t, token.ExpiresIn, stored.ExpiresIn, "expires_in should match")
		assert.Equal(t, token.TokenType, stored.TokenType, "token type should match")

		expectedExpiry := before.Add(time.Duration(token.ExpiresIn) * time.Second)
		assert.Equal(t, expectedExpiry.Format(time.RFC3339), stored.ExpiresAt.Format(time.RFC3339), "expiration time should match expected value")
	})

	t.Run("overwrites existing file if token already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		initialToken := TokenResponse{
			AccessToken:  "old-token",
			RefreshToken: "old-refresh",
			ExpiresIn:    1800,
			TokenType:    "Bearer",
		}
		err := s.save(initialToken)
		require.NoError(t, err)

		newToken := TokenResponse{
			AccessToken:  "new-token",
			RefreshToken: "new-refresh",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		err = s.save(newToken)
		require.NoError(t, err)

		data, err := os.ReadFile(tokenPath)
		require.NoError(t, err)

		var stored storedToken
		err = json.Unmarshal(data, &stored)
		require.NoError(t, err)

		assert.Equal(t, newToken.AccessToken, stored.AccessToken, "access token should be updated")
		assert.Equal(t, newToken.RefreshToken, stored.RefreshToken, "refresh token should be updated")
	})

	t.Run("error invalid path", func(t *testing.T) {
		s := &tokenStore{path: "/nonexistent/directory/token.json"}

		token := TokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
		}

		err := s.save(token)
		assert.Error(t, err, "save should fail for invalid path")
	})
}

func TestTokenStore_GetToken(t *testing.T) {
	t.Run("retrieves valid token", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		originalToken := TokenResponse{
			AccessToken:  "access-token-12345",
			RefreshToken: "refresh-token-67890",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		err := s.save(originalToken)
		require.NoError(t, err)

		retrievedToken, err := s.getToken()
		require.NoError(t, err, "getToken should succeed")

		assert.Equal(t, originalToken.AccessToken, retrievedToken.AccessToken, "access token should match")
		assert.Equal(t, originalToken.RefreshToken, retrievedToken.RefreshToken, "refresh token should match")
		assert.Equal(t, originalToken.ExpiresIn, retrievedToken.ExpiresIn, "expires_in should match")
		assert.Equal(t, originalToken.TokenType, retrievedToken.TokenType, "token type should match")
	})

	t.Run("token is not yet expired", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		futureToken := storedToken{
			TokenResponse: TokenResponse{
				AccessToken:  "future-token",
				RefreshToken: "refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		data, err := json.Marshal(futureToken)
		require.NoError(t, err)
		err = os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		token, err := s.getToken()
		require.NoError(t, err, "getToken should succeed for non-expired token")

		assert.Equal(t, futureToken.AccessToken, token.AccessToken, "access token should match")
		assert.Equal(t, futureToken.RefreshToken, token.RefreshToken, "refresh token should match")
	})

	t.Run("error token file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "nonexistent-token.json")
		s := &tokenStore{path: tokenPath}

		token, err := s.getToken()

		assert.Error(t, err, "getToken should fail when file doesn't exist")
		assert.True(t, os.IsNotExist(err), "error should be 'file does not exist'")
		assert.Empty(t, token.AccessToken, "token should be empty on error")
	})

	t.Run("error invalid JSON in token file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		err := os.WriteFile(tokenPath, []byte("not valid json"), 0o600)
		require.NoError(t, err)

		token, err := s.getToken()

		assert.Error(t, err, "getToken should fail for invalid JSON")
		assert.Empty(t, token.AccessToken, "token should be empty on error")
	})

	t.Run("error token is expired", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		expiredToken := storedToken{
			TokenResponse: TokenResponse{
				AccessToken:  "expired-token",
				RefreshToken: "refresh-token",
				ExpiresIn:    -3600, // past expiration
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(-1 * time.Hour), // 1 hour ago
		}

		data, err := json.Marshal(expiredToken)
		require.NoError(t, err)
		err = os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		token, err := s.getToken()

		assert.ErrorIs(t, err, ErrTokenExpired, "should return ErrTokenExpired for expired token")
		assert.Equal(t, expiredToken.AccessToken, token.AccessToken, "access token should match even if expired")
		assert.Equal(t, expiredToken.RefreshToken, token.RefreshToken, "refresh token should match even if expired")
		assert.Equal(t, expiredToken.ExpiresIn, token.ExpiresIn, "expires_in should match even if expired")
		assert.Equal(t, expiredToken.TokenType, token.TokenType, "token type should match even if expired")
	})

	t.Run("error malformed storedToken structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		s := &tokenStore{path: tokenPath}

		// write JSON with invalid time format
		malformedJSON := `{"access_token": "test", "expires_at": "not-a-valid-time"}`
		err := os.WriteFile(tokenPath, []byte(malformedJSON), 0o600)
		require.NoError(t, err)

		token, err := s.getToken()
		assert.Error(t, err, "getToken should fail for malformed JSON")
		assert.Empty(t, token.AccessToken, "access token should be empty")
		assert.Empty(t, token.RefreshToken, "refresh token should be empty")
	})
}
