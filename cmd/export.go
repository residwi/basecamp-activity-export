package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/residwi/bcreview/internal/aggregator"
	"github.com/residwi/bcreview/internal/api"
	"github.com/residwi/bcreview/internal/cache"
	"github.com/residwi/bcreview/internal/config"
	"github.com/residwi/bcreview/internal/export"
	"github.com/residwi/bcreview/internal/oauth"
	"github.com/spf13/cobra"
)

var exportFlags struct {
	accountID string
	year      int
	noCache   bool
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export performance review document",
	Long:  "Generates a comprehensive performance review document based on Basecamp activity data",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.MustLoad()

		if _, err := os.Stat(config.TokenFile()); os.IsNotExist(err) {
			return fmt.Errorf("no stored token found; please login first")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		oauthClient := oauth.NewClient(cfg)
		token, err := oauthClient.EnsureValidToken(ctx)
		if err != nil {
			return fmt.Errorf("ensuring valid token: %w", err)
		}

		var cacheManager cache.Manager
		if !exportFlags.noCache {
			cacheManager, err = cache.NewFileManager(config.CacheDir())
			if err != nil {
				return fmt.Errorf("initializing cache: %w", err)
			}
		}

		client := api.NewClient(exportFlags.accountID, token.AccessToken, cacheManager)
		agg := aggregator.NewAggregator(client)

		activity, err := agg.AggregateUserActivity(context.Background(), exportFlags.year)
		if err != nil {
			return fmt.Errorf("aggregating activity: %w", err)
		}

		filename := fmt.Sprintf("performance-review-%d.md", exportFlags.year)
		fmt.Printf("\nGenerating comprehensive performance review document...\n")
		if err := export.ExportPerformanceReview(activity, filename); err != nil {
			return fmt.Errorf("exporting performance review: %w", err)
		}

		fmt.Printf("\nPerformance review exported to: %s\n\n", filename)
		fmt.Println("This comprehensive report includes:")
		fmt.Println("  • Full descriptions with HTML converted to Markdown")
		fmt.Println("  • Direct links to all items in Basecamp")
		fmt.Println("  • Your comments and contributions on each item")
		fmt.Printf("  • Organized by Epic\n\n")

		summary := activity.Summary()

		fmt.Printf("\n")
		fmt.Printf("====================================================\n")
		fmt.Printf("  Basecamp Activity Summary - %d\n", activity.Year)
		fmt.Printf("====================================================\n")
		fmt.Printf("  User: %s\n", activity.User.Name)
		fmt.Printf("====================================================\n")
		fmt.Printf("  Messages Created:          %d\n", summary.TotalMessages)
		fmt.Printf("  Todos Created:             %d\n", summary.TotalTodos)
		fmt.Printf("  Todos Completed:           %d\n", summary.CompletedTodos)
		fmt.Printf("  Todolists Created:         %d\n", summary.TotalTodolists)
		fmt.Printf("  Comments Written:          %d\n", summary.TotalComments)
		fmt.Printf("  Daily Check-in Answers:    %d\n", summary.TotalQuestionAnswers)
		fmt.Printf("====================================================\n\n")

		fmt.Println("\nAPI Rate Limit Info:")
		fmt.Printf("  • Rate Limit Hits: %d\n", client.GetRateLimitHits())

		return nil
	},
}

func init() {
	cfg := config.MustLoad()

	exportCmd.Flags().StringVar(&exportFlags.accountID, "account-id", cfg.Basecamp.AccountID, "Basecamp account ID")
	exportCmd.Flags().IntVar(&exportFlags.year, "year", time.Now().Year(), "Year to fetch activity for")
	exportCmd.Flags().BoolVar(&exportFlags.noCache, "no-cache", false, "Disable HTTP caching")

	exportCmd.MarkPersistentFlagRequired("account-id")

	rootCmd.AddCommand(exportCmd)
}
