package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/config"
)

const (
	AuthURL  = "https://launchpad.37signals.com/authorization/new"
	TokenURL = "https://launchpad.37signals.com/authorization/token"
)

type oauthConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type OAuthClient struct {
	config         oauthConfig
	tokenStore     *tokenStore
	httpClient     *http.Client
	callbackServer *callbackServer
}

func NewOAuthClient(cfg config.Config) *OAuthClient {
	return &OAuthClient{
		config: oauthConfig{
			BaseURL:      "https://launchpad.37signals.com",
			ClientID:     cfg.Basecamp.ClientID,
			ClientSecret: cfg.Basecamp.ClientSecret,
			RedirectURI:  cfg.Basecamp.RedirectURI,
		},
		tokenStore:     &tokenStore{path: config.TokenFile()},
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		callbackServer: &callbackServer{port: 27072},
	}
}

// EnsureValidToken checks if a valid token exists, refreshing it if necessary
func (c *OAuthClient) EnsureValidToken(ctx context.Context) (TokenResponse, error) {
	token, err := c.tokenStore.getToken()
	if err == nil {
		return token, nil
	}

	if errors.Is(err, ErrTokenExpired) && token.RefreshToken != "" {
		fmt.Println("Refreshing expired token...")
		return c.refreshToken(ctx, token.RefreshToken)
	}

	return TokenResponse{}, fmt.Errorf("no valid token found, please login first")
}

// Authorize initiates the OAuth flow and returns an access token
func (c *OAuthClient) Authorize(ctx context.Context, state string) (TokenResponse, error) {
	authURL := c.buildAuthURL(state)

	fmt.Println("\nOAuth Authorization Required")
	fmt.Println("----------------------------------------")
	fmt.Printf("Please open this URL in your browser:\n\n%s\n\n", authURL)
	fmt.Println("Waiting for authorization...")

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go c.callbackServer.start(state, codeChan, errChan)

	select {
	case code := <-codeChan:
		return c.exchangeCode(ctx, code)
	case err := <-errChan:
		return TokenResponse{}, fmt.Errorf("callback error: %w", err)
	case <-ctx.Done():
		return TokenResponse{}, fmt.Errorf("authorization timeout")
	}
}

// exchangeCode exchanges the authorization code for an access token
func (c *OAuthClient) exchangeCode(ctx context.Context, code string) (TokenResponse, error) {
	params := url.Values{
		"type":          {"web_server"},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"redirect_uri":  {c.config.RedirectURI},
		"code":          {code},
	}

	tokenURL := c.buildTokenURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, nil)
	if err != nil {
		return TokenResponse{}, err
	}
	req.URL.RawQuery = params.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TokenResponse{}, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return TokenResponse{}, err
	}

	if err := c.tokenStore.save(token); err != nil {
		fmt.Printf("Warning: failed to cache token: %v\n", err)
	}

	return token, nil
}

// refreshToken uses the refresh token to obtain a new access token
func (c *OAuthClient) refreshToken(ctx context.Context, refreshToken string) (TokenResponse, error) {
	params := url.Values{
		"type":          {"refresh"},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"refresh_token": {refreshToken},
	}

	tokenURL := c.buildTokenURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, nil)
	if err != nil {
		return TokenResponse{}, err
	}
	req.URL.RawQuery = params.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TokenResponse{}, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return TokenResponse{}, err
	}

	if err := c.tokenStore.save(token); err != nil {
		fmt.Printf("Warning: failed to update cached token: %v\n", err)
	}

	return token, nil
}

func (c *OAuthClient) buildAuthURL(state string) string {
	authURL, _ := url.Parse(c.config.BaseURL)
	authURL = authURL.JoinPath("authorization", "new")

	q := authURL.Query()
	q.Set("type", "web_server")
	q.Set("client_id", c.config.ClientID)
	q.Set("redirect_uri", c.config.RedirectURI)
	q.Set("state", state)

	authURL.RawQuery = q.Encode()

	return authURL.String()
}

func (c *OAuthClient) buildTokenURL() string {
	tokenURL, _ := url.Parse(c.config.BaseURL)
	tokenURL = tokenURL.JoinPath("authorization", "token")

	return tokenURL.String()
}
