package cmd

import (
	"fmt"

	"github.com/residwi/bcreview/internal/cache"
	"github.com/residwi/bcreview/internal/config"
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
		cacheManager, err := cache.NewFileManager(config.CacheDir())
		if err != nil {
			return fmt.Errorf("initializing cache manager: %w", err)
		}

		if err := cacheManager.Clear(cmd.Context()); err != nil {
			return fmt.Errorf("clearing cache: %w", err)
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
		cacheManager, err := cache.NewFileManager(config.CacheDir())
		if err != nil {
			return fmt.Errorf("initializing cache manager: %w", err)
		}
		stats, err := cacheManager.Stats(cmd.Context())
		if err != nil {
			return fmt.Errorf("getting cache stats: %w", err)
		}

		fmt.Printf("Cache Statistics:\n")
		fmt.Printf("  Location: %s\n", config.CacheDir())
		fmt.Printf("  Total Entries: %d\n", stats.TotalEntries)
		fmt.Printf("  Total Size: %.2f MB\n", float64(stats.TotalSize)/(1024*1024))

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
