package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/residwi/bcreview/internal/config"
	"github.com/residwi/bcreview/internal/oauth"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Basecamp and store access token",
	Long:  "Initiates OAuth 2.0 flow to authenticate with Basecamp. Opens browser for authorization.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.MustLoad()

		if _, err := os.Stat(config.TokenFile()); err == nil {
			fmt.Println("You are already logged in.")
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		state := generateState()

		oauthClient := oauth.NewClient(cfg)
		token, err := oauthClient.Authorize(ctx, state)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		fmt.Println("\nSuccessfully authenticated")
		fmt.Printf("Access token saved (expires in %d seconds)\n", token.ExpiresIn)
		fmt.Println("You can now run the export command to generate your performance review.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

// generateState creates a random state value for CSRF protection
func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
