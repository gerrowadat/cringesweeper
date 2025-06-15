package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type PlatformStatus struct {
	Name             string            `json:"name"`
	Username         string            `json:"username"`
	LastPruneTime    time.Time         `json:"last_prune_time"`
	LastPruneStatus  string            `json:"last_prune_status"`
	LastPruneError   string            `json:"last_prune_error"`
	TotalRuns        int64             `json:"total_runs"`
	SuccessfulRuns   int64             `json:"successful_runs"`
	PostsProcessed   map[string]int64  `json:"posts_processed"`
	IsPruning        bool              `json:"is_pruning"`
	NextPruneTime    time.Time         `json:"next_prune_time"`
}

type ServerState struct {
	mu                sync.RWMutex
	Platforms         map[string]*PlatformStatus `json:"platforms"`
	StartTime         time.Time                  `json:"start_time"`
	Version           map[string]string          `json:"version"`
	PruneInterval     time.Duration              `json:"prune_interval"`
	DryRun            bool                       `json:"dry_run"`
}

func (s *ServerState) UpdatePlatformStatus(platform string, status *PlatformStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Platforms[platform] = status
}

func (s *ServerState) GetPlatformStatus(platform string) (*PlatformStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, exists := s.Platforms[platform]
	return status, exists
}

func (s *ServerState) GetAllPlatformStatuses() map[string]*PlatformStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*PlatformStatus)
	for k, v := range s.Platforms {
		result[k] = v
	}
	return result
}

type PlatformConfig struct {
	name     string
	username string
	client   internal.SocialClient
}

type PlatformRunner struct {
	Config  PlatformConfig
	Options internal.PruneOptions
}

var (
	// Global server state
	serverState *ServerState
	
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
	
	platformActiveGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cringesweeper_platform_active",
			Help: "Whether a platform is currently active (1) or not (0)",
		},
		[]string{"platform"},
	)
	
	platformPruningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cringesweeper_platform_pruning",
			Help: "Whether a platform is currently pruning (1) or not (0)",
		},
		[]string{"platform"},
	)
)

func init() {
	// Initialize server state
	serverState = &ServerState{
		Platforms: make(map[string]*PlatformStatus),
		StartTime: time.Now(),
	}
	
	// Register metrics
	prometheus.MustRegister(pruneRunsTotal)
	prometheus.MustRegister(postsProcessedTotal)
	prometheus.MustRegister(pruneRunDuration)
	prometheus.MustRegister(lastPruneTime)
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(versionInfo)
	prometheus.MustRegister(platformActiveGauge)
	prometheus.MustRegister(platformPruningGauge)
}

var serverCmd = &cobra.Command{
	Use:   "server [username]",
	Short: "Run as a long-term service with periodic pruning and metrics",
	Long: `Run CringeSweeper as a server that periodically prunes posts and serves metrics.

Use --platforms to monitor multiple platforms concurrently (e.g., --platforms=bluesky,mastodon
or --platforms=all). Each platform runs in its own goroutine with independent scheduling.

This mode runs continuously and:
- Periodically executes prune operations for each platform based on the configured interval
- Serves HTTP endpoints for health checks and Prometheus metrics with multi-platform status
- Suitable for containerized deployments for automated post management across platforms

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
		platformsStr, _ := cmd.Flags().GetString("platforms")
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

		// Determine which platforms to use
		var platforms []string
		
		if platformsStr == "" {
			fmt.Printf("Error: --platforms flag is required. Specify comma-separated platforms (bluesky,mastodon) or 'all'\n")
			os.Exit(1)
		}
		
		platforms, err = internal.ParsePlatforms(platformsStr)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Get username with fallback priority: argument > environment variables only (no saved credentials)
		argUsername := ""
		if len(args) > 0 {
			argUsername = args[0]
		}

		// Validate credentials for all platforms
		var platformConfigs []PlatformConfig
		for _, platformName := range platforms {
			username, err := internal.GetUsernameForPlatformEnvOnly(platformName, argUsername)
			if err != nil {
				fmt.Printf("Error for %s: %v\n", platformName, err)
				fmt.Printf("In server mode, credentials must be provided via environment variables only.\n")
				os.Exit(1)
			}

			client, exists := internal.GetClient(platformName)
			if !exists {
				fmt.Printf("Error: Unsupported platform '%s'. Supported platforms: %s\n", 
					platformName, strings.Join(internal.GetAllPlatformNames(), ", "))
				os.Exit(1)
			}
			
			platformConfigs = append(platformConfigs, PlatformConfig{
				name:     platformName,
				username: username,
				client:   client,
			})
		}

		// Create platform-specific configurations
		for i, config := range platformConfigs {
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
				switch config.name {
				case "mastodon":
					rateLimitDelay = 60 * time.Second
				case "bluesky":
					rateLimitDelay = 1 * time.Second
				default:
					rateLimitDelay = 5 * time.Second
				}
			}

			// Parse options for this platform
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
				fmt.Printf("Error for %s: Must specify either --max-post-age or --before-date\n", config.name)
				os.Exit(1)
			}

			// Verify credentials work before starting server
			if err := verifyCredentials(config.client, config.name); err != nil {
				fmt.Printf("Error: Failed to verify credentials for %s: %v\n", config.name, err)
				fmt.Printf("In server mode, credentials must be provided via environment variables.\n")
				os.Exit(1)
			}

			// Update platform config with options
			platformConfigs[i] = PlatformConfig{
				name:     config.name,
				username: config.username,
				client:   config.client,
			}

			log.Info().
				Str("platform", config.name).
				Str("username", config.username).
				Dur("prune_interval", pruneInterval).
				Int("port", port).
				Bool("dry_run", dryRun).
				Msg("Configured platform for CringeSweeper server")
		}

		log.Info().
			Int("platforms", len(platformConfigs)).
			Dur("prune_interval", pruneInterval).
			Int("port", port).
			Bool("dry_run", dryRun).
			Msg("Starting CringeSweeper multi-platform server")

		// Initialize server state
		serverState.PruneInterval = pruneInterval
		serverState.DryRun = dryRun
		serverState.Version = internal.GetFullVersionInfo()
		
		// Initialize platform statuses
		for _, config := range platformConfigs {
			serverState.UpdatePlatformStatus(config.name, &PlatformStatus{
				Name:           config.name,
				Username:       config.username,
				LastPruneStatus: "pending",
				PostsProcessed: make(map[string]int64),
				NextPruneTime:  time.Now(),
			})
			platformActiveGauge.WithLabelValues(config.name).Set(1)
		}
		
		// Create platform configurations with their specific options
		var platformRunners []PlatformRunner
		for _, config := range platformConfigs {
			// Create platform-specific options
			var rateLimitDelay time.Duration
			if rateLimitDelayStr != "" {
				delay, err := parseDuration(rateLimitDelayStr)
				if err != nil {
					fmt.Printf("Error parsing rate-limit-delay: %v\n", err)
					os.Exit(1)
				}
				rateLimitDelay = delay
			} else {
				switch config.name {
				case "mastodon":
					rateLimitDelay = 60 * time.Second
				case "bluesky":
					rateLimitDelay = 1 * time.Second
				default:
					rateLimitDelay = 5 * time.Second
				}
			}
			
			options := internal.PruneOptions{
				PreserveSelfLike: preserveSelfLike,
				PreservePinned:   preservePinned,
				UnlikePosts:      unlikePosts,
				UnshareReposts:   unshareReposts,
				DryRun:           dryRun,
				RateLimitDelay:   rateLimitDelay,
			}
			
			if maxAgeStr != "" {
				maxAge, err := parseDuration(maxAgeStr)
				if err != nil {
					fmt.Printf("Error parsing max-post-age: %v\n", err)
					os.Exit(1)
				}
				options.MaxAge = &maxAge
			}
			
			if beforeDateStr != "" {
				beforeDate, err := parseDate(beforeDateStr)
				if err != nil {
					fmt.Printf("Error parsing before-date: %v\n", err)
					os.Exit(1)
				}
				options.BeforeDate = &beforeDate
			}
			
			platformRunners = append(platformRunners, PlatformRunner{
				Config:  config,
				Options: options,
			})
		}
		
		// Start the multi-platform server
		startMultiPlatformServer(platformRunners, pruneInterval, port)
	},
}

func verifyCredentials(client internal.SocialClient, platform string) error {
	// Try to get credentials to verify they exist and are valid
	_, err := internal.GetCredentialsForPlatformEnvOnly(platform)
	return err
}

func startMultiPlatformServer(platformRunners []PlatformRunner, pruneInterval time.Duration, port int) {
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

		platformStatuses := serverState.GetAllPlatformStatuses()
		versionInfo := serverState.Version
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>CringeSweeper Multi-Platform Server</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; margin: 40px; }
        .status { padding: 10px; border-radius: 5px; margin: 10px 0; }
        .running { background-color: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .info { background-color: #d1ecf1; color: #0c5460; border: 1px solid #bee5eb; }
        .version { background-color: #f8f9fa; color: #495057; border: 1px solid #dee2e6; }
        .platform { border: 1px solid #ddd; margin: 10px 0; padding: 15px; border-radius: 5px; }
        .platform-header { font-weight: bold; font-size: 1.1em; margin-bottom: 10px; }
        .platform-status { display: inline-block; padding: 3px 8px; border-radius: 3px; font-size: 0.9em; }
        .status-success { background-color: #d4edda; color: #155724; }
        .status-error { background-color: #f8d7da; color: #721c24; }
        .status-pending { background-color: #fff3cd; color: #856404; }
        .status-pruning { background-color: #cce5ff; color: #004085; }
        .metrics-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 10px; margin: 10px 0; }
        .metric { background-color: #f8f9fa; padding: 8px; border-radius: 3px; text-align: center; }
        code { background-color: #f8f9fa; padding: 2px 4px; border-radius: 3px; }
        table { border-collapse: collapse; width: 100%%; margin: 10px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f8f9fa; }
    </style>
    <script>
        setTimeout(function() { location.reload(); }, 30000); // Auto-refresh every 30 seconds
    </script>
</head>
<body>
    <h1>🧹 CringeSweeper Multi-Platform Server</h1>
    <div class="status running">
        <strong>Status:</strong> Running (%d platforms active)
    </div>
    <div class="version">
        <p><strong>Version:</strong> %s</p>
        <p><strong>Commit:</strong> %s</p>
        <p><strong>Build Time:</strong> %s</p>
        <p><strong>Uptime:</strong> %v</p>
        <p><strong>Prune Interval:</strong> %v</p>
        <p><strong>Dry Run Mode:</strong> %t</p>
    </div>
    
    <h2>Platform Status</h2>`, len(platformStatuses), versionInfo["version"], versionInfo["commit"], versionInfo["build_time"], time.Since(serverState.StartTime).Round(time.Second), serverState.PruneInterval, serverState.DryRun)
		
		// Platform status sections
		for _, platform := range platformStatuses {
			statusClass := "status-pending"
			statusText := platform.LastPruneStatus
			if platform.IsPruning {
				statusClass = "status-pruning"
				statusText = "pruning"
			} else if platform.LastPruneStatus == "success" {
				statusClass = "status-success"
			} else if platform.LastPruneStatus == "error" {
				statusClass = "status-error"
			}
			
			fmt.Fprintf(w, `
    <div class="platform">
        <div class="platform-header">
            <span>%s</span>
            <span class="platform-status %s">%s</span>
        </div>
        <table>
            <tr><th>Username</th><td>%s</td></tr>
            <tr><th>Total Runs</th><td>%d</td></tr>
            <tr><th>Successful Runs</th><td>%d</td></tr>
            <tr><th>Last Prune</th><td>%s</td></tr>
            <tr><th>Next Prune</th><td>%s</td></tr>
        </table>
        <div class="metrics-grid">
            <div class="metric"><strong>Deleted</strong><br>%d</div>
            <div class="metric"><strong>Unliked</strong><br>%d</div>
            <div class="metric"><strong>Unshared</strong><br>%d</div>
            <div class="metric"><strong>Preserved</strong><br>%d</div>
        </div>`, 
				platform.Name, statusClass, statusText, platform.Username, 
				platform.TotalRuns, platform.SuccessfulRuns,
				formatTime(platform.LastPruneTime), formatTime(platform.NextPruneTime),
				platform.PostsProcessed["deleted"], platform.PostsProcessed["unliked"],
				platform.PostsProcessed["unshared"], platform.PostsProcessed["preserved"])
			
			if platform.LastPruneError != "" {
				fmt.Fprintf(w, `<div class="status-error" style="margin-top: 10px; padding: 8px;"><strong>Last Error:</strong> %s</div>`, platform.LastPruneError)
			}
			
			fmt.Fprintf(w, `
    </div>`)
		}
		
		fmt.Fprintf(w, `
    <h3>Endpoints</h3>
    <ul>
        <li><code>GET /</code> - This multi-platform status page (auto-refreshes every 30s)</li>
        <li><code>GET /metrics</code> - Prometheus metrics</li>
        <li><code>GET /api/status</code> - JSON status endpoint</li>
    </ul>
    <h3>Prometheus Metrics</h3>
    <p>Multi-platform metrics are available at <a href="/metrics">/metrics</a></p>
    <p>Key metrics include:</p>
    <ul>
        <li><code>cringesweeper_prune_runs_total{platform, status}</code> - Total prune runs per platform</li>
        <li><code>cringesweeper_posts_processed_total{platform, action}</code> - Posts processed by platform and action</li>
        <li><code>cringesweeper_prune_run_duration_seconds{platform}</code> - Prune run duration per platform</li>
        <li><code>cringesweeper_last_prune_timestamp{platform}</code> - Last prune timestamp per platform</li>
        <li><code>cringesweeper_platform_active{platform}</code> - Platform active status</li>
        <li><code>cringesweeper_platform_pruning{platform}</code> - Platform currently pruning status</li>
    </ul>
</body>
</html>`)

		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("HTTP request served")
	})
	
	// JSON API endpoint for programmatic access
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		status := "200"
		defer func() {
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		}()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		serverState.mu.RLock()
		jsonData, err := json.Marshal(serverState)
		serverState.mu.RUnlock()
		
		if err != nil {
			status = "500"
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Failed to marshal status: %v"}`, err)
			return
		}
		
		w.Write(jsonData)
		
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("JSON API request served")
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

	// Start platform monitoring goroutines
	var wg sync.WaitGroup
	platformCtxs := make(map[string]context.Context)
	platformCancels := make(map[string]context.CancelFunc)
	
	// Start each platform in its own goroutine
	for _, runner := range platformRunners {
		platformCtx, platformCancel := context.WithCancel(ctx)
		platformCtxs[runner.Config.name] = platformCtx
		platformCancels[runner.Config.name] = platformCancel
		
		wg.Add(1)
		go func(runner PlatformRunner) {
			defer wg.Done()
			startPlatformMonitoring(platformCtx, runner, pruneInterval)
		}(runner)
		
		log.Info().
			Str("platform", runner.Config.name).
			Str("username", runner.Config.username).
			Msg("Started platform monitoring goroutine")
	}

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Info().Msg("CringeSweeper server started successfully")

	// Main server loop
	select {
	case err := <-serverErrCh:
		log.Error().Err(err).Msg("HTTP server error")
		cancel()

	case sig := <-sigCh:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		
		// Cancel all platform contexts
		for platform, platformCancel := range platformCancels {
			log.Info().Str("platform", platform).Msg("Stopping platform monitoring")
			platformCancel()
		}
		
		// Graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}
		
		// Wait for platform goroutines to finish
		log.Info().Msg("Waiting for platform monitoring to complete...")
		wg.Wait()
		
		log.Info().Msg("Server shutdown complete")
		return
	}
	
	// Wait for platform goroutines to finish
	wg.Wait()
}

// Helper function to format time for display
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("2006-01-02 15:04:05 UTC")
}

// startPlatformMonitoring runs platform-specific monitoring in a dedicated goroutine
func startPlatformMonitoring(ctx context.Context, runner PlatformRunner, pruneInterval time.Duration) {
	platform := runner.Config.name
	username := runner.Config.username
	client := runner.Config.client
	options := runner.Options
	
	log.Info().Str("platform", platform).Msg("Platform monitoring started")
	
	// Create platform-specific ticker
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()
	
	// Platform-specific mutex to prevent concurrent pruning
	var pruningMutex sync.Mutex
	
	// Run initial prune
	go func() {
		pruningMutex.Lock()
		defer pruningMutex.Unlock()
		runPruneWithMetrics(client, username, options, platform)
	}()
	
	for {
		select {
		case <-ctx.Done():
			log.Info().Str("platform", platform).Msg("Platform monitoring stopped")
			// Update platform status to inactive
			platformActiveGauge.WithLabelValues(platform).Set(0)
			return
			
		case <-ticker.C:
			// Update next prune time
			if status, exists := serverState.GetPlatformStatus(platform); exists {
				status.NextPruneTime = time.Now().Add(pruneInterval)
				serverState.UpdatePlatformStatus(platform, status)
			}
			
			// Run prune in background to not block ticker
			go func() {
				if !pruningMutex.TryLock() {
					log.Warn().Str("platform", platform).Msg("Skipping prune run - previous run still in progress")
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
	errorMsg := ""

	log.Info().Str("platform", platform).Msg("Starting scheduled prune run")
	
	// Update platform status to indicate pruning is in progress
	if platformStatus, exists := serverState.GetPlatformStatus(platform); exists {
		platformStatus.IsPruning = true
		platformStatus.TotalRuns++
		serverState.UpdatePlatformStatus(platform, platformStatus)
		platformPruningGauge.WithLabelValues(platform).Set(1)
	}

	defer func() {
		duration := time.Since(start)
		pruneRunDuration.WithLabelValues(platform).Observe(duration.Seconds())
		pruneRunsTotal.WithLabelValues(platform, status).Inc()
		lastPruneTime.WithLabelValues(platform).Set(float64(time.Now().Unix()))
		
		// Update platform status
		if platformStatus, exists := serverState.GetPlatformStatus(platform); exists {
			platformStatus.IsPruning = false
			platformStatus.LastPruneTime = time.Now()
			platformStatus.LastPruneStatus = status
			platformStatus.LastPruneError = errorMsg
			if status == "success" {
				platformStatus.SuccessfulRuns++
			}
			platformStatus.NextPruneTime = time.Now().Add(serverState.PruneInterval)
			serverState.UpdatePlatformStatus(platform, platformStatus)
		}
		platformPruningGauge.WithLabelValues(platform).Set(0)
		
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
		errorMsg = err.Error()
		log.Error().Err(err).Str("platform", platform).Msg("Prune run failed")
		return
	}

	// Update metrics
	postsProcessedTotal.WithLabelValues(platform, "deleted").Add(float64(result.DeletedCount))
	postsProcessedTotal.WithLabelValues(platform, "unliked").Add(float64(result.UnlikedCount))
	postsProcessedTotal.WithLabelValues(platform, "unshared").Add(float64(result.UnsharedCount))
	postsProcessedTotal.WithLabelValues(platform, "preserved").Add(float64(result.PreservedCount))
	
	// Update platform status with post counts
	if platformStatus, exists := serverState.GetPlatformStatus(platform); exists {
		if platformStatus.PostsProcessed == nil {
			platformStatus.PostsProcessed = make(map[string]int64)
		}
		platformStatus.PostsProcessed["deleted"] += int64(result.DeletedCount)
		platformStatus.PostsProcessed["unliked"] += int64(result.UnlikedCount)
		platformStatus.PostsProcessed["unshared"] += int64(result.UnsharedCount)
		platformStatus.PostsProcessed["preserved"] += int64(result.PreservedCount)
		serverState.UpdatePlatformStatus(platform, platformStatus)
	}

	log.Info().
		Str("platform", platform).
		Int("deleted", result.DeletedCount).
		Int("unliked", result.UnlikedCount).
		Int("unshared", result.UnsharedCount).
		Int("preserved", result.PreservedCount).
		Int("errors", result.ErrorsCount).
		Msg("Prune run metrics")
}

// runContinuousPruneForServer runs continuous pruning with accurate success counting (server version of performContinuousPruningWithResult)
func runContinuousPruneForServer(client internal.SocialClient, username string, options internal.PruneOptions) (*internal.PruneResult, error) {
	// For server mode, we want to actually perform deletions (not dry-run)
	// and only count posts that were successfully processed
	serverOptions := options
	serverOptions.DryRun = false // Ensure we actually perform operations
	
	log.Debug().Str("platform", client.GetPlatformName()).Msg("Starting prune operation for server")
	
	// Use the platform's built-in PrunePosts method which correctly tracks successful operations
	result, err := client.PrunePosts(username, serverOptions)
	if err != nil {
		return nil, fmt.Errorf("prune operation failed: %w", err)
	}
	
	log.Debug().
		Str("platform", client.GetPlatformName()).
		Int("successfully_deleted", result.DeletedCount).
		Int("successfully_unliked", result.UnlikedCount).
		Int("successfully_unshared", result.UnsharedCount).
		Int("preserved", result.PreservedCount).
		Int("errors", result.ErrorsCount).
		Msg("Prune operation completed")
	
	return result, nil
}

func init() {
	rootCmd.AddCommand(serverCmd)
	
	// Server-specific flags
	serverCmd.Flags().IntP("port", "P", 8080, "HTTP server port")
	serverCmd.Flags().String("prune-interval", "1h", "Time between prune runs (e.g., 30m, 1h, 2h)")
	
	// Inherit all prune flags
	serverCmd.Flags().String("platforms", "", "Comma-separated list of platforms (bluesky,mastodon) or 'all' for all platforms")
	serverCmd.Flags().String("max-post-age", "", "Delete posts older than this (e.g., 30d, 1y, 24h)")
	serverCmd.Flags().String("before-date", "", "Delete posts created before this date (YYYY-MM-DD or MM/DD/YYYY)")
	serverCmd.Flags().Bool("preserve-selflike", false, "Don't delete user's own posts that they have liked")
	serverCmd.Flags().Bool("preserve-pinned", false, "Don't delete pinned posts")
	serverCmd.Flags().Bool("unlike-posts", false, "Unlike posts instead of deleting them")
	serverCmd.Flags().Bool("unshare-reposts", false, "Unshare/unrepost instead of deleting reposts")
	serverCmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting (for testing)")
	serverCmd.Flags().String("rate-limit-delay", "", "Delay between API requests to respect rate limits (default: 60s for Mastodon, 1s for Bluesky)")
}