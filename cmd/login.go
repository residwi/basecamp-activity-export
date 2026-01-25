package cmd

import (
	"context"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/command/auth"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Basecamp and store access token",
	Long:  "Initiates OAuth 2.0 flow to authenticate with Basecamp. Opens browser for authorization.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()

		return auth.Login(ctx)
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
