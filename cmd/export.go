package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/residwi/basecamp-activity-export/internal/command/auth"
	"github.com/residwi/basecamp-activity-export/internal/command/export"
	"github.com/residwi/basecamp-activity-export/internal/config"
	"github.com/residwi/basecamp-activity-export/internal/httpcache"
	"github.com/residwi/basecamp-activity-export/internal/progress"
	"github.com/residwi/basecamp-activity-export/internal/trace"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

var exportFlags struct {
	accountID string
	year      int
	noCache   bool
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export activity report",
	Long:  "Generates a comprehensive activity report based on Basecamp activity data",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.MustLoad()

		if _, err := os.Stat(config.TokenFile()); os.IsNotExist(err) {
			return fmt.Errorf("no stored token found; please login first")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()

		oauthClient := auth.NewOAuthClient(cfg)
		token, err := oauthClient.EnsureValidToken(ctx)
		if err != nil {
			return fmt.Errorf("ensuring valid token: %w", err)
		}

		limiter := rate.NewLimiter(rate.Limit(5), 10)
		var cacheManager httpcache.Manager
		if !exportFlags.noCache {
			cacheManager, err = httpcache.NewFileManager(config.CacheDir())
			if err != nil {
				return fmt.Errorf("initializing cache: %w", err)
			}
		}

		// Optimize transport for concurrent requests to the same host (default 20)
		// avoids connection churn from the default of 2 during bursts
		baseTransport := &http.Transport{
			MaxIdleConns:        cfg.ConcurrencyLimit,
			MaxIdleConnsPerHost: cfg.ConcurrencyLimit,
			IdleConnTimeout:     90 * time.Second,
		}
		httpClient := &http.Client{
			Timeout: 30 * time.Second,
			Transport: basecamp.ChainTransport(
				baseTransport,
				basecamp.Tracking(),
				basecamp.Cache(cacheManager),
				basecamp.Retry(5),
				basecamp.RateLimit(limiter),
			),
		}

		baseURL := fmt.Sprintf("https://3.basecampapi.com/%s", exportFlags.accountID)
		client := basecamp.NewClient(
			basecamp.WithHTTPClient(httpClient),
			basecamp.WithAccountID(exportFlags.accountID),
			basecamp.WithAccessToken(token.AccessToken),
			basecamp.WithBaseURL(baseURL),
			basecamp.WithUserAgent(cfg.Basecamp.UserAgent),
			basecamp.WithConcurrencyLimit(cfg.ConcurrencyLimit),
		)

		tracker := trace.NewTracer()
		filename := fmt.Sprintf("activity-%d.md", exportFlags.year)
		exporter := export.New(
			export.WithBasecampClient(client),
			export.WithTracker(tracker),
			export.WithFilename(filename),
			export.WithDisplay(progress.NewDisplay(tracker)),
		)
		activity, err := exporter.FetchActivity(context.Background(), exportFlags.year)
		if err != nil {
			return fmt.Errorf("exporting formatted activity: %w", err)
		}
		fmt.Printf("\n====================================================\n")
		fmt.Printf("  Basecamp Activity Summary - %d\n", activity.Year())
		fmt.Printf("====================================================\n")
		fmt.Printf("  User: %s\n", activity.User().Name)
		fmt.Printf("====================================================\n")
		fmt.Printf("  Messages Created:          %d\n", len(activity.MessagesCreated()))
		fmt.Printf("  Todos Created:             %d\n", len(activity.TodosCreated()))
		fmt.Printf("  Todos Completed:           %d\n", len(activity.TodosCompleted()))
		fmt.Printf("  Todolists Created:         %d\n", len(activity.TodolistsCreated()))
		fmt.Printf("  Comments Written:          %d\n", len(activity.CommentsWritten()))
		fmt.Printf("  Daily Check-in Answers:    %d\n", len(activity.QuestionAnswers()))
		fmt.Printf("====================================================\n\n")

		fmt.Printf("Generating comprehensive activity report...\n")
		if err := export.FormatAndSaveReport(activity, filename); err != nil {
			return fmt.Errorf("formatting and saving report: %w", err)
		}
		fmt.Printf("\nActivity report exported to: %s\n", filename)
		fmt.Println("This comprehensive report includes:")
		fmt.Println("  • Full descriptions with HTML converted to Markdown")
		fmt.Println("  • Direct links to all items in Basecamp")
		fmt.Println("  • Your comments and activity on each item")
		fmt.Printf("  • Organized by project\n\n")

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
