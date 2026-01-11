package cmd

import (
	"fmt"
	"os"

	"github.com/residwi/bcreview/internal/config"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication token",
	Long:  "Removes the cached OAuth token, requiring re-authentication on next use.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := os.Remove(config.TokenFile())
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No stored token found; already logged out")
				return nil
			}
			return fmt.Errorf("removing token file: %w", err)
		}

		fmt.Println("Logged out successfully")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
