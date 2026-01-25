package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_Login(t *testing.T) {
	t.Run("skips login if already logged in", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		configDir := filepath.Join(tmpDir, ".bclog")
		err := os.MkdirAll(configDir, 0o755)
		require.NoError(t, err)

		tokenPath := filepath.Join(configDir, "token.json")

		validToken := storedToken{
			TokenResponse: TokenResponse{
				AccessToken:  "existing-token",
				RefreshToken: "refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		data, _ := json.Marshal(validToken)
		err = os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		err = Login(t.Context())
		require.NoError(t, err)
	})

	t.Run("performs oauth flow if not logged in with authentication failed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			jsonResponse, _ := json.Marshal(TokenResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			})
			w.Write(jsonResponse)
		}))
		defer tokenServer.Close()

		ctx := t.Context()
		go func() {
			// use a loop to ensure the request is made after server starts
			for {
				select {
				case <-ctx.Done():
					return // exit goroutine if test is done
				default:
					http.Get("http://localhost:27072/callback?code=auth-code-123&state=invalid-state")
				}
			}
		}()

		err := Login(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "authentication failed", "should return authentication failed error")
	})
}

func TestAuth_Logout(t *testing.T) {
	t.Run("removes token file", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		configDir := filepath.Join(tmpDir, ".bclog")
		err := os.MkdirAll(configDir, 0o755)
		require.NoError(t, err)

		tokenPath := filepath.Join(configDir, "token.json")

		validToken := storedToken{
			TokenResponse: TokenResponse{
				AccessToken:  "existing-token",
				RefreshToken: "refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		data, _ := json.Marshal(validToken)
		err = os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		err = Logout()
		require.NoError(t, err)

		_, err = os.Stat(tokenPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("handles missing token file gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		err := Logout()
		require.NoError(t, err, "logout should succeed even if token file does not exist")
	})
}

func TestGenerateState(t *testing.T) {
	t.Run("generates non-empty state", func(t *testing.T) {
		state := generateState()
		assert.NotEmpty(t, state, "state should not be empty")
	})

	t.Run("generates unique states", func(t *testing.T) {
		state1 := generateState()
		state2 := generateState()
		assert.NotEqual(t, state1, state2, "consecutive states should be different")
	})
}
