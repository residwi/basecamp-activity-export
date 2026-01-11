package oauth

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

var ErrTokenExpired = errors.New("stored token is expired")

type storedToken struct {
	tokenResponse
	ExpiresAt time.Time `json:"expires_at"`
}

type tokenStore struct {
	path string
}

// save persists a token to disk
func (s *tokenStore) save(token tokenResponse) error {
	stored := &storedToken{
		tokenResponse: token,
		ExpiresAt:     time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o600)
}

// getToken retrieves a stored token if it exists and is not expired
func (s *tokenStore) getToken() (tokenResponse, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return tokenResponse{}, err
	}

	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		return tokenResponse{}, err
	}

	if time.Now().After(stored.ExpiresAt) {
		return stored.tokenResponse, ErrTokenExpired
	}

	return stored.tokenResponse, nil
}
