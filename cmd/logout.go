package cmd

import (
	"github.com/residwi/basecamp-activity-export/internal/command/auth"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication token",
	Long:  "Removes the cached OAuth token, requiring re-authentication on next use.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return auth.Logout()
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
