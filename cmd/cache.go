package cmd

import (
	"fmt"

	"github.com/residwi/basecamp-activity-export/internal/command/cache"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage HTTP cache",
	Long:  "Manage HTTP cache for Basecamp API responses to improve performance",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached data",
	Long:  "Removes all cached API responses, forcing fresh requests on next run",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cache.Clear(cmd.Context()); err != nil {
			return err
		}

		fmt.Println("Cache cleared successfully")
		return nil
	},
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  "Display information about cached API responses including size and entry count",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats, err := cache.Stats(cmd.Context())
		if err != nil {
			return err
		}

		fmt.Printf("Cache Statistics:\n")
		fmt.Printf("  Location: %s\n", stats.CacheDir)
		fmt.Printf("  Total Entries: %d\n", stats.TotalFiles)
		fmt.Printf("  Total Size: %s\n", stats.TotalSizeFormatted)

		if stats.OldestEntry != nil {
			fmt.Printf("  Oldest Entry: %s\n", stats.OldestEntry.Format("2006-01-02 15:04:05"))
		}
		if stats.NewestEntry != nil {
			fmt.Printf("  Newest Entry: %s\n", stats.NewestEntry.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	rootCmd.AddCommand(cacheCmd)
}
