package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bclog",
	Short: "Basecamp Activity Export Tool",
	Long: `A comprehensive CLI tool to fetch and export your Basecamp activity data.
	Generates activity reports with complete activity history.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
