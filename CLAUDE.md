# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building
```bash
go build  # Builds the main binary as 'cringesweeper'
```

Note: The binary is excluded from version control via .gitignore to follow Go best practices.

### Testing
```bash
go test ./...        # Run all tests (380+ tests, all passing)
go test ./internal   # Run tests for internal package only
go vet ./...         # Check for common Go issues
```

All tests are fully implemented and passing. The test suite provides comprehensive coverage of platform implementations, authentication, post processing, and command functionality.

### Code Formatting and Quality
```bash
gofmt -w .  # Format all Go code
go vet ./... # Validate code quality
```

**Important**: Always run `go vet ./...` before committing to ensure no unused code or formatting issues.

## Architecture Overview

CringeSweeper is a CLI tool for managing social media posts across multiple platforms (Bluesky, Mastodon) with comprehensive multi-platform support.

### Core Architecture

The codebase follows a clean separation of concerns:

- **`cmd/`** - Cobra CLI commands (auth, ls, prune, server, root) with multi-platform support
- **`internal/`** - Core business logic and platform implementations
- **`cringesweeper.go`** - Main entry point

### Key Components

1. **SocialClient Interface** (`internal/social.go`): 
   - Defines common operations for all social platforms
   - Implemented by BlueskyClient and MastodonClient
   - Supports post fetching, pruning, pagination, and authentication requirements
   - Key methods: `FetchUserPosts()`, `FetchUserPostsPaginated()`, `PrunePosts()`

2. **Multi-Platform Management** (`internal/social.go`):
   - `ParsePlatforms()`: Parses comma-separated platform lists and validates them
   - `GetAllPlatformNames()`: Returns all supported platform names
   - Support for "all" keyword to operate on all platforms
   - Platform validation with helpful error messages

2. **Post Structure** (`internal/social.go`):
   - Unified post representation across platforms
   - Supports different post types: original, repost, reply, like, quote
   - Includes engagement metrics and platform-specific metadata

3. **Authentication System** (`internal/auth.go`, `internal/credentials.go`):
   - Multi-layered credential resolution: CLI args → saved files → environment variables
   - Platform-specific credential validation
   - Credentials stored in `~/.config/cringesweeper/`
   - Session management with caching and refresh token support

4. **Logging System** (`internal/logger.go`):
   - Comprehensive zerolog-based logging throughout the application
   - DEBUG: All HTTP requests and authentication operations with URL redaction
   - INFO: Write operations (deletes, unlikes, unshares) with content previews
   - ERROR: Unexpected external failures
   - Configurable via `--log-level` flag or `LOG_LEVEL` environment variable

5. **Pagination and Streaming** (`cmd/ls.go`, `cmd/prune.go`):
   - Protocol-native pagination: AT Protocol cursors (Bluesky), max_id (Mastodon)
   - Streaming output for real-time user feedback during long operations
   - Smart early termination to avoid unnecessary API calls when age thresholds are reached

6. **Pruning Engine** (`internal/social.go`):
   - Configurable deletion criteria (age, date, preservation rules)
   - Safety features: dry-run mode, preserve pinned/self-liked posts
   - Alternative actions: unlike posts, unshare reposts instead of deletion
   - Continuous processing mode with `--continue` flag for exhaustive operations

### Platform Implementations

Each social platform implements the `SocialClient` interface:

- **Bluesky** (`internal/bsky.go`): Uses app passwords, faster rate limits (1s default)
- **Mastodon** (`internal/mastodon.go`): Uses OAuth2 tokens, conservative rate limits (60s default)

### Multi-Platform Server Architecture

The server mode (`cmd/server.go`) implements a sophisticated concurrent architecture for multi-platform monitoring:

#### Goroutine Architecture
- **Main HTTP Server**: Serves web interface, JSON API, and Prometheus metrics without blocking
- **Platform Monitoring Goroutines**: Each platform runs in its own dedicated goroutine with independent:
  - Pruning schedules and intervals
  - Rate limiting and API throttling
  - Error handling and retry logic
  - Metrics collection and reporting

#### Server State Management
- **Thread-Safe State**: `ServerState` struct with mutex protection for concurrent access
- **Per-Platform Status**: Real-time tracking of pruning status, metrics, and health per platform
- **Live Updates**: Platform statuses update in real-time without blocking other platforms

#### Prometheus Metrics
- **Platform-Specific Metrics**: All metrics include platform labels for multi-platform observability
- **Active Platform Tracking**: `cringesweeper_platform_active` gauge indicates platform operational status
- **Pruning Status**: `cringesweeper_platform_pruning` gauge shows real-time pruning activity
- **Independent Scheduling**: Each platform maintains separate run counts, durations, and success rates

#### Web Interface
- **Multi-Platform Dashboard**: Shows status for all configured platforms simultaneously
- **Auto-Refresh**: Page automatically refreshes every 30 seconds for live monitoring
- **Per-Platform Metrics**: Detailed statistics and status for each platform
- **JSON API**: `/api/status` endpoint provides programmatic access to complete server state

#### Graceful Shutdown
- **Coordinated Shutdown**: Cleanly stops all platform goroutines on termination signals
- **Resource Cleanup**: Properly cancels contexts and waits for completion of in-progress operations
- **Status Updates**: Updates platform metrics to reflect shutdown state

### Adding New Platforms

To add support for a new social platform:

1. Implement `SocialClient` interface in `internal/`
2. Add authentication flow in `cmd/auth.go` 
3. Register platform in `SupportedPlatforms` map in `internal/social.go`
4. Add credential validation logic in `internal/credentials.go`

### Command Structure

All commands support multi-platform operations:

- **`auth`**: Platform authentication setup and credential management
  - `--platforms`: Set up authentication for multiple platforms (bluesky,mastodon or "all")
- **`ls`**: List recent posts from user timelines with filtering and streaming
  - `--platforms`: List posts from multiple platforms simultaneously 
  - `--continue`: Search entire timeline instead of just recent posts
  - `--max-post-age`: Filter posts by age (e.g., "30d", "1y")
  - `--before-date`: Filter posts before specific date
  - `--limit`: Control batch size for pagination
- **`prune`**: Delete/unlike/unshare posts based on criteria with continuous processing
  - `--platforms`: Prune posts across multiple platforms with combined results
  - `--continue`: Process entire timeline instead of just recent posts
  - `--dry-run`: Preview actions without performing them
  - All age filtering options from `ls` command
- **`server`**: Long-term service mode with concurrent multi-platform pruning and metrics
  - `--platforms`: Monitor multiple platforms concurrently with dedicated goroutines per platform
  - Full multi-platform server support with independent scheduling and monitoring
  - `--port`: HTTP server port (default: 8080)
  - `--prune-interval`: Time between prune runs for each platform (default: 1h)
  - All prune flags supported for periodic operations across all platforms
  - Credentials ONLY from environment variables (no config files)
  - Serves comprehensive multi-platform metrics at `/metrics` endpoint
  - Real-time platform status at `/` with auto-refresh and detailed per-platform information
  - JSON API endpoint at `/api/status` for programmatic access to multi-platform status

All commands support:
- `--platforms` flag for multi-platform operations (comma-separated list or "all")
- `--log-level` flag for debugging (debug, info, warn, error)

### Testing

The test suite includes:
- Unit tests for credential management and validation
- Social client interface compliance tests
- Post type and pruning logic validation

Tests use table-driven patterns and may require environment setup for some credential-related tests.

## Containerization and Deployment

### Docker Support

CringeSweeper includes full Docker support for containerized deployments:

- **`Dockerfile`**: Multi-stage build with Alpine Linux for minimal image size with version info
- **`.dockerignore`**: Optimized build context
- **`.env.example`**: Template for environment configuration

### Multi-Platform Server Mode for Containers

The `server` command is specifically designed for long-term containerized deployment with full multi-platform support:

- **Concurrent Multi-Platform Processing**: Each platform runs in its own goroutine with independent scheduling
- **Environment-only credentials**: No config files, only environment variables for all platforms
- **Multi-Platform Health Checks**: Built-in HTTP health endpoint at `/` with per-platform status dashboard
- **Comprehensive Prometheus Metrics**: Platform-specific metrics at `/metrics` with full observability
- **JSON API**: `/api/status` endpoint for programmatic multi-platform status access
- **Graceful Multi-Platform Shutdown**: Coordinated shutdown of all platform goroutines
- **Non-root execution**: Runs as unprivileged user for security
- **Resource efficient**: Optimized goroutine architecture suitable for constrained environments
- **Real-time Updates**: Web interface auto-refreshes with live platform status without blocking

### Quick Start with Docker

```bash
# Build with version information
docker build --build-arg VERSION=0.0.1 --build-arg COMMIT=$(git rev-parse HEAD) --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) -t cringesweeper .

# Run multi-platform server
docker run -d -p 8080:8080 --env-file .env cringesweeper server --platforms=bluesky,mastodon --max-post-age=30d

# Run server with all platforms
docker run -d -p 8080:8080 --env-file .env cringesweeper server --platforms=all --max-post-age=30d --prune-interval=2h
```

### Version Management

CringeSweeper uses semantic versioning with git tags:
- **Version info**: Available at `/` endpoint and in Prometheus metrics
- **Build-time injection**: Version, commit, and build time embedded during Docker build
- **Git tag integration**: Automatically detects version from git tags when available

## Development Guidelines

### Code Quality Standards
- Always run `go vet ./...` before committing to catch unused functions and variables
- Use `gofmt -w .` for consistent formatting
- Remove any dead code immediately to prevent VSCode warnings
- Follow Go naming conventions and keep methods concise

### HTTP Logging Requirements  
- **Every HTTP request must be logged at DEBUG level** using `LogHTTPRequest()` and `LogHTTPResponse()`
- URLs must be automatically redacted for sensitive information (passwords, tokens, JWTs)
- Include request method, full URL, and response status in logs

### Streaming and User Experience
- Use streaming output for operations that may take time (large lists, continuous processing)
- Provide real-time feedback during multi-round operations
- Display progress indicators and round numbers for continuous processing
- Show immediate results rather than waiting for batch completion

### API Efficiency
- Implement smart early termination when age criteria are reached
- Use protocol-native pagination to avoid infinite loops
- Respect rate limits with platform-appropriate delays (1s Bluesky, 60s Mastodon)
- Stop fetching when no more relevant content will be found

### Error Handling
- Log all HTTP failures at ERROR level with context
- Provide user-friendly error messages with actionable guidance
- Continue gracefully when possible, fail fast when necessary
- Include platform and operation context in error messages