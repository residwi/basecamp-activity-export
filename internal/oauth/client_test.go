package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/residwi/bcreview/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("creates client with correct config", func(t *testing.T) {
		cfg := config.Config{
			Basecamp: config.Basecamp{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  "http://localhost:8080/callback",
			},
		}

		client := NewClient(cfg)

		require.NotNil(t, client, "client should not be nil")
		assert.Equal(t, cfg.Basecamp.ClientID, client.config.ClientID, "client ID should match")
		assert.Equal(t, cfg.Basecamp.ClientSecret, client.config.ClientSecret, "client secret should match")
		assert.Equal(t, cfg.Basecamp.RedirectURI, client.config.RedirectURI, "redirect URI should match")
		assert.Equal(t, config.TokenFile(), client.tokenStore.path, "token store path should match")
		assert.Equal(t, 27072, client.callbackServer.port, "callback server should use port 27072")
		assert.Equal(t, 30*time.Second, client.httpClient.Timeout, "HTTP client should have 30s timeout")
	})
}

func TestOAuthClient_EnsureValidToken(t *testing.T) {
	t.Run("returns valid token from store", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		validToken := storedToken{
			tokenResponse: tokenResponse{
				AccessToken:  "stored-access-token",
				RefreshToken: "stored-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		data, _ := json.Marshal(validToken)
		err := os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		client := &client{
			tokenStore: &tokenStore{path: tokenPath},
		}
		token, err := client.EnsureValidToken(t.Context())
		require.NoError(t, err, "should return valid token without error")

		assert.Equal(t, validToken.AccessToken, token.AccessToken, "access token should match")
		assert.Equal(t, validToken.RefreshToken, token.RefreshToken, "refresh token should match")
	})

	t.Run("no stored token", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "nonexistent-token.json")

		client := &client{
			tokenStore: &tokenStore{path: tokenPath},
		}

		token, err := client.EnsureValidToken(t.Context())
		require.Error(t, err, "should return error when no token is stored")

		assert.EqualError(t, err, "no valid token found, please login first")
		assert.Empty(t, token, "token should be empty when none is stored")
	})

	t.Run("refreshes expired token", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		expiredStoredToken := storedToken{
			tokenResponse: tokenResponse{
				AccessToken:  "expired-access-token",
				RefreshToken: "valid-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		data, _ := json.Marshal(expiredStoredToken)
		err := os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "should use POST method")
			assert.Equal(t, "/authorization/token", r.URL.Path, "should hit token endpoint")

			query := r.URL.Query()
			assert.Equal(t, "refresh", query.Get("type"), "type should be refresh")
			assert.Equal(t, "test-client-id", query.Get("client_id"), "client_id should match")
			assert.Equal(t, "test-client-secret", query.Get("client_secret"), "client_secret should match")
			assert.Equal(t, "valid-refresh-token", query.Get("refresh_token"), "refresh_token should match")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			jsonResponse, _ := json.Marshal(tokenResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			})
			w.Write(jsonResponse)
		}))
		defer oauthServer.Close()

		callbackServerPort := getFreePort(t)
		client := &client{
			config: oauthConfig{
				BaseURL:      oauthServer.URL,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  fmt.Sprintf("http://localhost:%d/callback", callbackServerPort),
			},
			httpClient: &http.Client{},
			tokenStore: &tokenStore{path: tokenPath},
		}

		token, err := client.EnsureValidToken(t.Context())
		require.NoError(t, err, "should refresh token without error")

		assert.Equal(t, "new-access-token", token.AccessToken, "should return new access token")
		assert.Equal(t, "new-refresh-token", token.RefreshToken, "should return new refresh token")
		assert.Equal(t, 3600, token.ExpiresIn, "expires_in should be positive")
		assert.Equal(t, "Bearer", token.TokenType, "token type should be Bearer")
	})

	t.Run("error refreshing token with invalid refresh token", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		expiredStoredToken := storedToken{
			tokenResponse: tokenResponse{
				AccessToken:  "expired-access-token",
				RefreshToken: "invalid-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		data, _ := json.Marshal(expiredStoredToken)
		err := os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`invalid refresh token`))
		}))
		defer oauthServer.Close()

		client := &client{
			config: oauthConfig{
				BaseURL: oauthServer.URL,
			},
			httpClient: &http.Client{},
			tokenStore: &tokenStore{path: tokenPath},
		}
		token, err := client.EnsureValidToken(t.Context())
		require.Error(t, err, "should return error on failed refresh")

		assert.EqualError(t, err, "token refresh failed: invalid refresh token")
		assert.Empty(t, token, "token should be empty on error")
	})

	t.Run("error malformed JSON response during refresh", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")
		expiredStoredToken := storedToken{
			tokenResponse: tokenResponse{
				AccessToken:  "expired-access-token",
				RefreshToken: "valid-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		data, _ := json.Marshal(expiredStoredToken)
		err := os.WriteFile(tokenPath, data, 0o600)
		require.NoError(t, err)

		oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json}`))
		}))
		defer oauthServer.Close()

		client := &client{
			config: oauthConfig{
				BaseURL: oauthServer.URL,
			},
			httpClient: &http.Client{},
			tokenStore: &tokenStore{path: tokenPath},
		}
		token, err := client.EnsureValidToken(t.Context())
		require.Error(t, err, "should return error on malformed JSON")

		assert.Empty(t, token, "token should be empty on JSON parse error")
	})
}

func TestOAuthClient_Authorize(t *testing.T) {
	t.Run("success authorize and exchange code", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "should use POST method")
			assert.Equal(t, "/authorization/token", r.URL.Path, "should hit token endpoint")

			query := r.URL.Query()
			assert.Equal(t, "web_server", query.Get("type"), "type should be web_server")
			assert.Equal(t, "test-client-id", query.Get("client_id"), "client_id should match")
			assert.Equal(t, "test-client-secret", query.Get("client_secret"), "client_secret should match")
			assert.Equal(t, "auth-code-123", query.Get("code"), "code should be passed")
			assert.NotEmpty(t, query.Get("redirect_uri"), "redirect_uri should be present")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			jsonResponse, _ := json.Marshal(tokenResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			})
			w.Write(jsonResponse)
		}))
		defer tokenServer.Close()

		state := "expected-csrf-state"
		callbackServerPort := getFreePort(t)
		client := &client{
			config: oauthConfig{
				BaseURL:      tokenServer.URL,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  fmt.Sprintf("http://localhost:%d/callback", callbackServerPort),
			},
			httpClient:     &http.Client{},
			tokenStore:     &tokenStore{path: tokenPath},
			callbackServer: &callbackServer{port: callbackServerPort},
		}

		go func() {
			// use a loop to ensure the request is made after server starts
			for {
				select {
				case <-t.Context().Done():
					return // exit goroutine if test is done
				default:
					callbackURL := fmt.Sprintf("http://localhost:%d/callback?code=auth-code-123&state=%s", callbackServerPort, state)
					http.Get(callbackURL)
				}
			}
		}()

		token, err := client.Authorize(t.Context(), state)
		require.NoError(t, err, "should authorize without error")

		assert.Equal(t, "new-access-token", token.AccessToken, "should return new access token")
		assert.Equal(t, "new-refresh-token", token.RefreshToken, "should return new refresh token")
		assert.Equal(t, 3600, token.ExpiresIn, "expires_in should be positive")
		assert.Equal(t, "Bearer", token.TokenType, "token type should be Bearer")
		assert.FileExists(t, tokenPath, "token should be saved to store")
	})

	t.Run("invalid state parameter", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		callbackServerPort := getFreePort(t)
		client := &client{
			config: oauthConfig{
				BaseURL:      "https://test.example.com",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  fmt.Sprintf("http://localhost:%d/callback", callbackServerPort),
			},
			httpClient:     &http.Client{},
			tokenStore:     &tokenStore{path: tokenPath},
			callbackServer: &callbackServer{port: callbackServerPort},
		}

		state := "expected-csrf-state"
		go func() {
			// use a loop to ensure the request is made after server starts
			for {
				select {
				case <-t.Context().Done():
					return // exit goroutine if test is done
				default:
					callbackURL := fmt.Sprintf("http://localhost:%d/callback?code=auth-code-123&state=invalid-state", callbackServerPort)
					http.Get(callbackURL)
				}
			}
		}()

		token, err := client.Authorize(t.Context(), state)
		require.Error(t, err, "should return error on invalid state")

		assert.EqualError(t, err, "callback error: invalid state parameter")
		assert.Empty(t, token, "token should be empty on error")
	})

	t.Run("context timeout during authorization", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		callbackServerPort := getFreePort(t)
		client := &client{
			config: oauthConfig{
				BaseURL:      "https://test.example.com",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  fmt.Sprintf("http://localhost:%d/callback", callbackServerPort),
			},
			httpClient: &http.Client{},
			tokenStore: &tokenStore{path: tokenPath},
			callbackServer: &callbackServer{
				port: callbackServerPort,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		token, err := client.Authorize(ctx, "state-value")
		require.Error(t, err, "should return error on timeout")

		assert.EqualError(t, err, "authorization timeout")
		assert.Empty(t, token, "token should be empty on timeout")
	})

	t.Run("failed token exchange with non-200 status", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`stub-error-response`))
		}))
		defer tokenServer.Close()

		callbackServerPort := getFreePort(t)
		client := &client{
			config: oauthConfig{
				BaseURL:      tokenServer.URL,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  fmt.Sprintf("http://localhost:%d/callback", callbackServerPort),
			},
			httpClient:     &http.Client{},
			tokenStore:     &tokenStore{path: tokenPath},
			callbackServer: &callbackServer{port: callbackServerPort},
		}

		state := "expected-csrf-state"
		go func() {
			// use a loop to ensure the request is made after server starts
			for {
				select {
				case <-t.Context().Done():
					return // exit goroutine if test is done
				default:
					callbackURL := fmt.Sprintf("http://localhost:%d/callback?code=auth-code-123&state=%s", callbackServerPort, state)
					http.Get(callbackURL)
				}
			}
		}()

		token, err := client.Authorize(t.Context(), state)
		require.Error(t, err, "should return error on failed exchange")

		assert.EqualError(t, err, "token exchange failed: stub-error-response")
		assert.Empty(t, token, "token should be empty on error")
	})

	t.Run("malformed exchange JSON response", func(t *testing.T) {
		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "token.json")

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json}`))
		}))
		defer tokenServer.Close()

		callbackServerPort := getFreePort(t)
		client := &client{
			config: oauthConfig{
				BaseURL:      tokenServer.URL,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURI:  fmt.Sprintf("http://localhost:%d/callback", callbackServerPort),
			},
			httpClient:     &http.Client{},
			tokenStore:     &tokenStore{path: tokenPath},
			callbackServer: &callbackServer{port: callbackServerPort},
		}

		state := "expected-csrf-state"
		go func() {
			// use a loop to ensure the request is made after server starts
			for {
				select {
				case <-t.Context().Done():
					return // exit goroutine if test is done
				default:
					callbackURL := fmt.Sprintf("http://localhost:%d/callback?code=auth-code-123&state=%s", callbackServerPort, state)
					http.Get(callbackURL)
				}
			}
		}()

		token, err := client.Authorize(t.Context(), state)
		require.Error(t, err, "should return error on malformed JSON")

		assert.Empty(t, token, "token should be empty on JSON parse error")
	})
}

func TestOAuthClient_BuildAuthURL(t *testing.T) {
	t.Run("constructs correct URL with all parameters", func(t *testing.T) {
		client := &client{
			config: oauthConfig{
				BaseURL:     "https://custom-oauth.example.com",
				ClientID:    "test-client-id",
				RedirectURI: "http://localhost:8080/callback",
			},
		}

		state := "test-state-value"
		authURL := client.buildAuthURL(state)

		parsedURL, err := url.Parse(authURL)
		require.NoError(t, err, "URL should be valid")
		assert.Equal(t, "https", parsedURL.Scheme, "scheme should be https")
		assert.Equal(t, "custom-oauth.example.com", parsedURL.Host, "host should be correct")
		assert.Equal(t, "/authorization/new", parsedURL.Path, "path should be correct")

		query := parsedURL.Query()
		assert.Equal(t, "web_server", query.Get("type"), "type parameter should be web_server")
		assert.Equal(t, "test-client-id", query.Get("client_id"), "client_id should match")
		assert.Equal(t, "http://localhost:8080/callback", query.Get("redirect_uri"), "redirect_uri should match")
		assert.Equal(t, "test-state-value", query.Get("state"), "state should match")
	})
}

func TestOAuthClient_BuildTokenURL(t *testing.T) {
	t.Run("constructs correct token URL", func(t *testing.T) {
		client := &client{
			config: oauthConfig{
				BaseURL: "https://custom-oauth.example.com",
			},
		}

		tokenURL := client.buildTokenURL()
		expectedURL := "https://custom-oauth.example.com/authorization/token"
		assert.Equal(t, expectedURL, tokenURL, "token URL should match expected")
	})
}
