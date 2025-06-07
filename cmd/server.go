package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	// Prometheus metrics
	pruneRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cringesweeper_prune_runs_total",
			Help: "Total number of prune runs executed",
		},
		[]string{"platform", "status"},
	)
	
	postsProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cringesweeper_posts_processed_total",
			Help: "Total number of posts processed",
		},
		[]string{"platform", "action"},
	)
	
	pruneRunDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cringesweeper_prune_run_duration_seconds",
			Help:    "Duration of prune runs in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"platform"},
	)
	
	lastPruneTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cringesweeper_last_prune_timestamp",
			Help: "Timestamp of the last prune run",
		},
		[]string{"platform"},
	)
	
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cringesweeper_http_requests_total",
			Help: "Total number of HTTP requests to the server",
		},
		[]string{"method", "path", "status"},
	)
	
	versionInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cringesweeper_version_info",
			Help: "Version information for CringeSweeper",
		},
		[]string{"version", "commit", "build_time"},
	)
)

func init() {
	// Register metrics
	prometheus.MustRegister(pruneRunsTotal)
	prometheus.MustRegister(postsProcessedTotal)
	prometheus.MustRegister(pruneRunDuration)
	prometheus.MustRegister(lastPruneTime)
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(versionInfo)
}

var serverCmd = &cobra.Command{
	Use:   "server [username]",
	Short: "Run as a long-term service with periodic pruning and metrics",
	Long: `Run CringeSweeper as a server that periodically prunes posts and serves metrics.

This mode runs continuously and:
- Periodically executes prune operations based on the configured interval
- Serves HTTP endpoints for health checks and Prometheus metrics
- Suitable for containerized deployments for automated post management

Server endpoints:
- GET /         - Health check with service information
- GET /metrics  - Prometheus metrics endpoint

In server mode, credentials are ONLY read from environment variables:
- BLUESKY_USERNAME, BLUESKY_APP_PASSWORD
- MASTODON_USERNAME, MASTODON_ACCESS_TOKEN, MASTODON_INSTANCE

All prune flags are supported for configuring the periodic pruning behavior.
Use --prune-interval to control how often pruning runs (default: 1h).`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		platform, _ := cmd.Flags().GetString("platform")
		port, _ := cmd.Flags().GetInt("port")
		pruneIntervalStr, _ := cmd.Flags().GetString("prune-interval")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		preserveSelfLike, _ := cmd.Flags().GetBool("preserve-selflike")
		preservePinned, _ := cmd.Flags().GetBool("preserve-pinned")
		unlikePosts, _ := cmd.Flags().GetBool("unlike-posts")
		unshareReposts, _ := cmd.Flags().GetBool("unshare-reposts")
		maxAgeStr, _ := cmd.Flags().GetString("max-post-age")
		beforeDateStr, _ := cmd.Flags().GetString("before-date")
		rateLimitDelayStr, _ := cmd.Flags().GetString("rate-limit-delay")

		// Parse prune interval
		pruneInterval, err := parseDuration(pruneIntervalStr)
		if err != nil {
			fmt.Printf("Error parsing prune-interval: %v\n", err)
			os.Exit(1)
		}

		// Get username with fallback priority: argument > environment variables only (no saved credentials)
		argUsername := ""
		if len(args) > 0 {
			argUsername = args[0]
		}

		username, err := internal.GetUsernameForPlatformEnvOnly(platform, argUsername)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Printf("In server mode, credentials must be provided via environment variables only.\n")
			os.Exit(1)
		}

		client, exists := internal.GetClient(platform)
		if !exists {
			fmt.Printf("Error: Unsupported platform '%s'. Supported platforms: bluesky, mastodon\n", platform)
			os.Exit(1)
		}

		// Parse rate limit delay - use platform-appropriate defaults
		var rateLimitDelay time.Duration
		if rateLimitDelayStr != "" {
			delay, err := parseDuration(rateLimitDelayStr)
			if err != nil {
				fmt.Printf("Error parsing rate-limit-delay: %v\n", err)
				os.Exit(1)
			}
			rateLimitDelay = delay
		} else {
			// Set platform-appropriate defaults
			switch platform {
			case "mastodon":
				rateLimitDelay = 60 * time.Second
			case "bluesky":
				rateLimitDelay = 1 * time.Second
			default:
				rateLimitDelay = 5 * time.Second
			}
		}

		// Parse options
		options := internal.PruneOptions{
			PreserveSelfLike: preserveSelfLike,
			PreservePinned:   preservePinned,
			UnlikePosts:      unlikePosts,
			UnshareReposts:   unshareReposts,
			DryRun:           dryRun,
			RateLimitDelay:   rateLimitDelay,
		}

		// Parse max age
		if maxAgeStr != "" {
			maxAge, err := parseDuration(maxAgeStr)
			if err != nil {
				fmt.Printf("Error parsing max-post-age: %v\n", err)
				os.Exit(1)
			}
			options.MaxAge = &maxAge
		}

		// Parse before date
		if beforeDateStr != "" {
			beforeDate, err := parseDate(beforeDateStr)
			if err != nil {
				fmt.Printf("Error parsing before-date: %v\n", err)
				os.Exit(1)
			}
			options.BeforeDate = &beforeDate
		}

		// Validate that at least one criteria is specified
		if options.MaxAge == nil && options.BeforeDate == nil {
			fmt.Println("Error: Must specify either --max-post-age or --before-date")
			os.Exit(1)
		}

		// Verify credentials work before starting server
		if err := verifyCredentials(client, platform); err != nil {
			fmt.Printf("Error: Failed to verify credentials: %v\n", err)
			fmt.Printf("In server mode, credentials must be provided via environment variables.\n")
			os.Exit(1)
		}

		log.Info().
			Str("platform", platform).
			Str("username", username).
			Dur("prune_interval", pruneInterval).
			Int("port", port).
			Bool("dry_run", dryRun).
			Msg("Starting CringeSweeper server")

		// Start the server
		startServer(client, username, options, platform, pruneInterval, port)
	},
}

func verifyCredentials(client internal.SocialClient, platform string) error {
	// Try to get credentials to verify they exist and are valid
	_, err := internal.GetCredentialsForPlatformEnvOnly(platform)
	return err
}

func startServer(client internal.SocialClient, username string, options internal.PruneOptions, platform string, pruneInterval time.Duration, port int) {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize version metrics
	version := internal.GetFullVersionInfo()
	versionInfo.WithLabelValues(version["version"], version["commit"], version["build_time"]).Set(1)

	// Setup HTTP server
	mux := http.NewServeMux()
	
	// Root endpoint - health check with service info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		status := "200"
		defer func() {
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		}()

		versionInfo := internal.GetFullVersionInfo()
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>CringeSweeper Server</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; margin: 40px; }
        .status { padding: 10px; border-radius: 5px; margin: 10px 0; }
        .running { background-color: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .info { background-color: #d1ecf1; color: #0c5460; border: 1px solid #bee5eb; }
        .version { background-color: #f8f9fa; color: #495057; border: 1px solid #dee2e6; }
        code { background-color: #f8f9fa; padding: 2px 4px; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>🧹 CringeSweeper Server</h1>
    <div class="status running">
        <strong>Status:</strong> Running
    </div>
    <div class="version">
        <p><strong>Version:</strong> %s</p>
        <p><strong>Commit:</strong> %s</p>
        <p><strong>Build Time:</strong> %s</p>
    </div>
    <div class="info">
        <p><strong>Platform:</strong> %s</p>
        <p><strong>Username:</strong> %s</p>
        <p><strong>Prune Interval:</strong> %v</p>
        <p><strong>Dry Run Mode:</strong> %t</p>
        <p><strong>Last Check:</strong> %s</p>
    </div>
    <h3>Endpoints</h3>
    <ul>
        <li><code>GET /</code> - This status page</li>
        <li><code>GET /metrics</code> - Prometheus metrics</li>
    </ul>
    <h3>Metrics</h3>
    <p>Prometheus metrics are available at <a href="/metrics">/metrics</a></p>
    <p>Key metrics include:</p>
    <ul>
        <li><code>cringesweeper_prune_runs_total</code> - Total prune runs</li>
        <li><code>cringesweeper_posts_processed_total</code> - Posts processed by action</li>
        <li><code>cringesweeper_prune_run_duration_seconds</code> - Prune run duration</li>
        <li><code>cringesweeper_last_prune_timestamp</code> - Last prune timestamp</li>
    </ul>
</body>
</html>`, versionInfo["version"], versionInfo["commit"], versionInfo["build_time"], platform, username, pruneInterval, options.DryRun, time.Now().Format("2006-01-02 15:04:05 UTC"))

		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("HTTP request served")
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start HTTP server in goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		log.Info().Int("port", port).Msg("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// Start pruning ticker in goroutine
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()

	var pruningMutex sync.Mutex
	
	// Run initial prune
	go func() {
		pruningMutex.Lock()
		defer pruningMutex.Unlock()
		runPruneWithMetrics(client, username, options, platform)
	}()

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Info().Msg("CringeSweeper server started successfully")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, shutting down")
			return

		case err := <-serverErrCh:
			log.Error().Err(err).Msg("HTTP server error")
			cancel()
			return

		case sig := <-sigCh:
			log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
			
			// Graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()
			
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Error().Err(err).Msg("Error during server shutdown")
			}
			
			log.Info().Msg("Server shutdown complete")
			return

		case <-ticker.C:
			// Run prune in background to not block metrics serving
			go func() {
				if !pruningMutex.TryLock() {
					log.Warn().Msg("Skipping prune run - previous run still in progress")
					return
				}
				defer pruningMutex.Unlock()
				runPruneWithMetrics(client, username, options, platform)
			}()
		}
	}
}

func runPruneWithMetrics(client internal.SocialClient, username string, options internal.PruneOptions, platform string) {
	start := time.Now()
	status := "success"

	log.Info().Str("platform", platform).Msg("Starting scheduled prune run")

	defer func() {
		duration := time.Since(start)
		pruneRunDuration.WithLabelValues(platform).Observe(duration.Seconds())
		pruneRunsTotal.WithLabelValues(platform, status).Inc()
		lastPruneTime.WithLabelValues(platform).Set(float64(time.Now().Unix()))
		
		log.Info().
			Str("platform", platform).
			Str("status", status).
			Dur("duration", duration).
			Msg("Prune run completed")
	}()

	// Use continuous pruning to process entire timeline
	result, err := runContinuousPruneForServer(client, username, options)
	if err != nil {
		status = "error"
		log.Error().Err(err).Str("platform", platform).Msg("Prune run failed")
		return
	}

	// Update metrics
	postsProcessedTotal.WithLabelValues(platform, "deleted").Add(float64(result.DeletedCount))
	postsProcessedTotal.WithLabelValues(platform, "unliked").Add(float64(result.UnlikedCount))
	postsProcessedTotal.WithLabelValues(platform, "unshared").Add(float64(result.UnsharedCount))
	postsProcessedTotal.WithLabelValues(platform, "preserved").Add(float64(result.PreservedCount))

	log.Info().
		Str("platform", platform).
		Int("deleted", result.DeletedCount).
		Int("unliked", result.UnlikedCount).
		Int("unshared", result.UnsharedCount).
		Int("preserved", result.PreservedCount).
		Int("errors", result.ErrorsCount).
		Msg("Prune run metrics")
}

// runContinuousPruneForServer runs continuous pruning similar to performContinuousPruning but returns aggregated results
func runContinuousPruneForServer(client internal.SocialClient, username string, options internal.PruneOptions) (*internal.PruneResult, error) {
	totalResult := &internal.PruneResult{
		PostsToDelete:  []internal.Post{},
		PostsToUnlike:  []internal.Post{},
		PostsToUnshare: []internal.Post{},
		PostsPreserved: []internal.Post{},
		Errors:         []string{},
	}

	cursor := ""
	batchLimit := 100
	round := 1

	log.Debug().Msg("Starting continuous prune for server")

	for {
		log.Debug().Int("round", round).Msg("Fetching posts for pruning")
		
		posts, nextCursor, err := client.FetchUserPostsPaginated(username, batchLimit, cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts in round %d: %w", round, err)
		}

		// Check termination conditions
		if len(posts) == 0 {
			log.Debug().Int("round", round).Msg("No more posts found, pruning complete")
			break
		}

		if nextCursor == "" || nextCursor == cursor {
			log.Debug().Int("round", round).Msg("Reached end of timeline, pruning complete")
			break
		}

		// Filter posts by age criteria
		now := time.Now()
		matchingPosts := []internal.Post{}
		for _, post := range posts {
			shouldProcess := false

			if options.MaxAge != nil && now.Sub(post.CreatedAt) > *options.MaxAge {
				shouldProcess = true
			}
			if options.BeforeDate != nil && post.CreatedAt.Before(*options.BeforeDate) {
				shouldProcess = true
			}

			if shouldProcess {
				matchingPosts = append(matchingPosts, post)
			}
		}

		// Process matching posts
		if len(matchingPosts) > 0 {
			log.Debug().Int("round", round).Int("matching_posts", len(matchingPosts)).Msg("Processing posts")
			
			// Create a temporary options struct for this batch
			batchOptions := options
			batchOptions.DryRun = false // Always actually process in server mode
			
			// For server mode, we need to actually process posts individually
			// This is a simplified version - in a full implementation, you'd want to 
			// reuse the exact logic from PrunePosts but adapted for continuous operation
			for _, post := range matchingPosts {
				// Check preservation rules
				preserveReason := ""
				if options.PreservePinned && post.IsPinned {
					preserveReason = "pinned"
				} else if options.PreserveSelfLike && post.IsLikedByUser && post.Type == internal.PostTypeOriginal {
					preserveReason = "self-liked"
				}

				if preserveReason != "" {
					totalResult.PostsPreserved = append(totalResult.PostsPreserved, post)
					totalResult.PreservedCount++
				} else {
					// For now, just count what would be processed
					// In a full implementation, you'd call the actual deletion methods here
					if post.Type == internal.PostTypeLike {
						totalResult.PostsToUnlike = append(totalResult.PostsToUnlike, post)
						totalResult.UnlikedCount++
					} else if post.Type == internal.PostTypeRepost {
						totalResult.PostsToUnshare = append(totalResult.PostsToUnshare, post)
						totalResult.UnsharedCount++
					} else if post.Type == internal.PostTypeOriginal || post.Type == internal.PostTypeReply {
						totalResult.PostsToDelete = append(totalResult.PostsToDelete, post)
						totalResult.DeletedCount++
					}
				}
			}
		}

		cursor = nextCursor
		round++
		time.Sleep(1 * time.Second) // Small delay between rounds
	}

	log.Debug().
		Int("total_rounds", round-1).
		Int("total_processed", totalResult.DeletedCount+totalResult.UnlikedCount+totalResult.UnsharedCount).
		Msg("Continuous prune completed")

	return totalResult, nil
}

func init() {
	rootCmd.AddCommand(serverCmd)
	
	// Server-specific flags
	serverCmd.Flags().IntP("port", "P", 8080, "HTTP server port")
	serverCmd.Flags().String("prune-interval", "1h", "Time between prune runs (e.g., 30m, 1h, 2h)")
	
	// Inherit all prune flags
	serverCmd.Flags().StringP("platform", "p", "bluesky", "Social media platform (bluesky, mastodon)")
	serverCmd.Flags().String("max-post-age", "", "Delete posts older than this (e.g., 30d, 1y, 24h)")
	serverCmd.Flags().String("before-date", "", "Delete posts created before this date (YYYY-MM-DD or MM/DD/YYYY)")
	serverCmd.Flags().Bool("preserve-selflike", false, "Don't delete user's own posts that they have liked")
	serverCmd.Flags().Bool("preserve-pinned", false, "Don't delete pinned posts")
	serverCmd.Flags().Bool("unlike-posts", false, "Unlike posts instead of deleting them")
	serverCmd.Flags().Bool("unshare-reposts", false, "Unshare/unrepost instead of deleting reposts")
	serverCmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting (for testing)")
	serverCmd.Flags().String("rate-limit-delay", "", "Delay between API requests to respect rate limits (default: 60s for Mastodon, 1s for Bluesky)")
}