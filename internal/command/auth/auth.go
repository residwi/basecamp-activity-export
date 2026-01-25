package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/residwi/basecamp-activity-export/internal/config"
)

// Login performs OAuth flow and stores the access token
func Login(ctx context.Context) error {
	cfg := config.MustLoad()
	oauthClient := NewOAuthClient(cfg)

	if _, err := oauthClient.tokenStore.getToken(); err == nil {
		fmt.Println("You are already logged in.")
		return nil
	}

	state := generateState()

	token, err := oauthClient.Authorize(ctx, state)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println("\nSuccessfully authenticated")
	fmt.Printf("Access token saved (expires in %d seconds)\n", token.ExpiresIn)
	fmt.Println("You can now run the export command to generate your activity report.")

	return nil
}

// Logout removes the stored access token
func Logout() error {
	err := os.Remove(config.TokenFile())
	if os.IsNotExist(err) {
		fmt.Println("No stored token found; already logged out")
		return nil
	}

	fmt.Println("Logged out successfully")

	return nil
}

// generateState creates a random state value for CSRF protection
func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
