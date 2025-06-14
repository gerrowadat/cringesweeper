# CringeSweeper 🧹

> **⚠️ Experimental Repository Disclaimer**
> 
> **I wrote none of this code.** This repository serves as an experimental exploration of using Claude AI to produce functional software through conversational programming. Despite my usual skepticism about AI-generated code (especially without deep expertise in the domain), I wanted to approach this with an open mind and see what's possible.
>
> The entire development process is documented in [`claude-conversation.md`](claude-conversation.md), which contains a log of nearly everything I asked Claude to implement to get the code to this state. This serves both as a transparency record and as a case study in AI-assisted development.
>
> Use this software at your own discretion and always test thoroughly before running on important data.

A command-line tool for managing your social media presence across multiple platforms. View, analyze, and selectively delete posts from Bluesky, Mastodon, and other social networks.

## Features

- **Multi-platform operations**: Use `--platforms=all` to operate on Bluesky and Mastodon simultaneously
- **Cross-platform management**: List, prune, and authenticate across multiple platforms in a single command
- **Post viewing**: List and browse your recent posts across platforms with streaming output
- **Intelligent pruning**: Delete, unlike, or unshare posts based on age, date, and smart criteria
- **Continuous processing**: Process entire post history with `--continue` flag using proper pagination
- **Streaming output**: See results immediately as posts are found and processed
- **Safety first**: Dry-run mode shows what would be deleted before actually deleting
- **Preserve important posts**: Keep pinned posts and self-liked content
- **Cross-platform authentication**: Guided setup for API keys and tokens across multiple platforms
- **Post type detection**: Distinguishes between original posts, reposts, replies, and quotes
- **Comprehensive logging**: Debug-level HTTP logging with sensitive data redaction
- **Server mode**: Long-term containerized deployment with Prometheus metrics

## Installation

### From Source

```bash
git clone https://github.com/gerrowadat/cringesweeper.git
cd cringesweeper

# Build with version information (recommended)
make build

# Quick development build (faster, no version info)
make build-dev

# Show all available build targets
make help
```

### Docker Image

```bash
# Build Docker image with version information
make docker-build

# Run server with environment variables
make docker-run BLUESKY_USERNAME=user.bsky.social BLUESKY_APP_PASSWORD=secret

# Or configure .env file and run
make env-example  # Copy template
# Edit .env with your credentials
make docker-run-env
```

### Prerequisites

- Go 1.24.2 or later (for source builds)
- Docker (for containerized deployment)
- Access to your social media accounts for authentication

## Quick Start

1. **Set up authentication** for your preferred platform:
   ```bash
   ./cringesweeper auth --platforms=bluesky
   ```

2. **View your recent posts** (uses saved credentials automatically):
   ```bash
   ./cringesweeper ls
   ```

3. **Preview what would be deleted** (safe dry-run):
   ```bash
   ./cringesweeper prune --max-post-age=30d --dry-run
   ```

4. **Check your authentication status for all platforms**:
   ```bash
   ./cringesweeper auth --status
   ```

## Global Flags

All commands support these global flags:

- `--log-level string`: Set the logging level (debug, info, warn, error) (default "info")
- `-h, --help`: Help for any command

**Logging Examples:**
```bash
# Enable debug logging to see all HTTP requests and responses
./cringesweeper --log-level=debug ls username

# Quiet mode - only show errors
./cringesweeper --log-level=error prune --dry-run --max-post-age=30d
```

## Commands

### `ls` - List Recent Posts

Display recent posts from a user's timeline with optional filtering and continuous searching. Supports multiple platforms for comprehensive social media management.

```bash
./cringesweeper ls [username] [flags]
```

**Flags:**
- `--platforms string`: **Required** - Comma-separated list of platforms (bluesky,mastodon) or 'all' for all platforms
- `--limit string`: Maximum number of posts to fetch per batch (default "10")
- `--max-post-age string`: Only show posts older than this (e.g., 30d, 1y, 24h)
- `--before-date string`: Only show posts created before this date (YYYY-MM-DD or MM/DD/YYYY)
- `--continue`: Continue searching and fetching posts until no more are found
- `-h, --help`: Help for ls command

**Examples:**
```bash
# List recent posts from Bluesky using saved credentials
./cringesweeper ls --platforms=bluesky

# List posts from a specific Bluesky user
./cringesweeper ls --platforms=bluesky user.bsky.social

# List posts from a Mastodon user
./cringesweeper ls --platforms=mastodon user@mastodon.social

# MULTI-PLATFORM: List posts from all platforms
./cringesweeper ls --platforms=all

# MULTI-PLATFORM: List posts from specific platforms
./cringesweeper ls --platforms=bluesky,mastodon

# Show posts older than 30 days with streaming output
./cringesweeper ls --platforms=bluesky --max-post-age=30d

# Continuously search through entire timeline for old posts
./cringesweeper ls --platforms=bluesky --continue --before-date=2023-01-01

# MULTI-PLATFORM: Search all platforms for old posts
./cringesweeper ls --platforms=all --continue --before-date=2023-01-01

# Search with custom batch size for faster processing
./cringesweeper ls --continue --limit=50 --max-post-age=1y

# Use environment variable for username (fallback method)
export BLUESKY_USER=user.bsky.social
./cringesweeper ls
```

**Streaming Output:**
- Posts are displayed immediately as they are found and filtered
- Use `--continue` to search through your entire post history
- Perfect for finding old posts or getting an overview of your posting patterns

**Username Resolution:**
CringeSweeper automatically finds your username using this priority order:
1. Username provided as command argument (highest priority)
2. Username from saved credentials (`~/.config/cringesweeper/`)
3. Username from environment variables
4. Generic `SOCIAL_USER` environment variable (lowest priority)

**Environment Variables:**
- `BLUESKY_USER`: Default Bluesky username
- `MASTODON_USER`: Default Mastodon username  
- `SOCIAL_USER`: Fallback username for any platform

### `prune` - Delete, Unlike, or Unshare Posts by Criteria

Delete, unlike, or unshare posts from your timeline based on age, date, and preservation rules. Supports multiple platforms for comprehensive social media cleanup.

Alternative actions include unliking posts or unsharing reposts instead of deleting them,
providing gentler cleanup options. Supports continuous processing to handle entire post histories.

```bash
./cringesweeper prune [username] [flags]
```

**Flags:**
- `--platforms string`: **Required** - Comma-separated list of platforms (bluesky,mastodon) or 'all' for all platforms
- `--max-post-age string`: Delete posts older than this (e.g., 30d, 1y, 24h)
- `--before-date string`: Delete posts created before this date (YYYY-MM-DD or MM/DD/YYYY)
- `--preserve-selflike`: Don't delete user's own posts that they have liked
- `--preserve-pinned`: Don't delete pinned posts
- `--unlike-posts`: Unlike posts instead of deleting them
- `--unshare-reposts`: Unshare/unrepost instead of deleting reposts
- `--continue`: Continue searching and processing posts until no more match the criteria
- `--rate-limit-delay string`: Delay between API requests to respect rate limits (default: 60s for Mastodon, 1s for Bluesky)
- `--dry-run`: Show what would be deleted without actually deleting
- `-h, --help`: Help for prune command

**Duration Formats:**
- `h` - hours (e.g., `24h`)
- `d` - days (e.g., `30d`)
- `w` - weeks (e.g., `2w`)
- `m` - months (e.g., `6m`)
- `y` - years (e.g., `1y`)

**Date Formats:**
- `2006-01-02` (YYYY-MM-DD)
- `2006-01-02 15:04:05` (YYYY-MM-DD HH:MM:SS)
- `01/02/2006` (MM/DD/YYYY)
- `01/02/2006 15:04:05` (MM/DD/YYYY HH:MM:SS)
- ISO 8601 formats with timezone

**Examples:**
```bash
# Safe preview using saved credentials (RECOMMENDED FIRST STEP)
./cringesweeper prune --max-post-age=30d --dry-run

# Delete posts older than 1 year, preserve pinned and self-liked
./cringesweeper prune --max-post-age=1y --preserve-pinned --preserve-selflike

# MULTI-PLATFORM: Prune posts from all platforms
./cringesweeper prune --platforms=all --max-post-age=30d --dry-run

# MULTI-PLATFORM: Prune posts from specific platforms
./cringesweeper prune --platforms=bluesky,mastodon --max-post-age=30d --dry-run

# Unlike posts instead of deleting them
./cringesweeper prune --max-post-age=90d --unlike-posts --dry-run

# Unshare reposts instead of deleting them
./cringesweeper prune --max-post-age=30d --unshare-reposts --dry-run

# Combined approach: unlike liked posts, unshare reposts, delete the rest
./cringesweeper prune --max-post-age=6m --unlike-posts --unshare-reposts --dry-run

# Delete posts before a specific date for specific user
./cringesweeper prune --before-date="2023-01-01" --dry-run user.bsky.social

# CONTINUOUS PROCESSING - Process entire post history (NEW FEATURE)
./cringesweeper prune --continue --max-post-age=1y --dry-run
./cringesweeper prune --continue --before-date="2023-01-01" --preserve-pinned --dry-run

# MULTI-PLATFORM CONTINUOUS - Process all platforms continuously
./cringesweeper prune --platforms=all --continue --max-post-age=1y --dry-run

# Mastodon pruning with multiple criteria
./cringesweeper prune --platforms=mastodon --max-post-age=6m --preserve-selflike

# Bluesky pruning (uses 1s default delay for faster processing)
./cringesweeper prune --platforms=bluesky --max-post-age=30d --dry-run

# Mastodon pruning (uses 60s default delay for API compliance)
./cringesweeper prune --platforms=mastodon --max-post-age=30d --dry-run

# Custom rate limiting to override platform defaults
./cringesweeper prune --platforms=bluesky --max-post-age=30d --rate-limit-delay=500ms --dry-run
```

**Continuous Processing (`--continue` flag):**
- **Default behavior**: Processes recent posts only (typically ~100 most recent)
- **With `--continue`**: Searches through your entire post history until no more posts match criteria
- **Streaming output**: Shows posts immediately as they're found and processed
- **Proper pagination**: Uses platform-native pagination to avoid infinite loops
- **Progress tracking**: Shows round-by-round progress with cumulative statistics
- **Safe termination**: Automatically stops when reaching the end of your timeline

**Rate Limiting:**
- **Mastodon**: Default 60 seconds between requests (30 DELETE requests per 30 minutes limit)
- **Bluesky**: Default 1 second between requests (5,000 operations per hour, more permissive)
- Platform-specific defaults automatically applied based on selected platform
- Use `--rate-limit-delay` to override defaults (e.g., `30s`, `2m`, `5s`)

**⚠️ Safety Notes:**
- **Always use `--dry-run` first** to preview what actions will be performed
- Post deletion is **permanent** and cannot be undone
- Unlike and unshare operations are reversible (you can re-like or re-share)
- Authentication is required for all pruning operations
- Rate limiting prevents API violations but increases processing time

### `auth` - Setup Authentication

Guide you through setting up authentication credentials for social media platforms. Supports multiple platforms for streamlined setup.

```bash
./cringesweeper auth [flags]
```

**Flags:**
- `--platforms string`: Comma-separated list of platforms (bluesky,mastodon) or 'all' for all platforms
- `--status`: Show credential status for all platforms
- `-h, --help`: Help for auth command

**Examples:**
```bash
# Setup Bluesky authentication
./cringesweeper auth --platforms=bluesky

# Setup Mastodon authentication
./cringesweeper auth --platforms=mastodon

# MULTI-PLATFORM: Setup authentication for all platforms
./cringesweeper auth --platforms=all

# MULTI-PLATFORM: Setup authentication for specific platforms
./cringesweeper auth --platforms=bluesky,mastodon

# Check credential status for all platforms
./cringesweeper auth --status
```

### `server` - Long-term Service Mode

Run CringeSweeper as a persistent service with periodic pruning and Prometheus metrics. Designed for containerized deployments.

```bash
./cringesweeper server [username] [flags]
```

**Server-specific Flags:**
- `-P, --port int`: HTTP server port (default 8080)
- `--prune-interval string`: Time between prune runs (e.g., 30m, 1h, 2h) (default "1h")
- `--platforms string`: **Required** - Comma-separated list of platforms (bluesky,mastodon) or 'all' for all platforms
- All `prune` command flags are supported for periodic operations

**Note:** Multi-platform server support is currently in development. The server will use the first specified platform only.

**Important:** In server mode, credentials are ONLY read from environment variables (no config files).

**Server Endpoints:**
- `GET /`: Health check with service information
- `GET /metrics`: Prometheus metrics endpoint

**Key Metrics Exported:**
- `cringesweeper_prune_runs_total`: Total number of prune runs
- `cringesweeper_posts_processed_total`: Posts processed by action type
- `cringesweeper_prune_run_duration_seconds`: Duration of prune operations
- `cringesweeper_last_prune_timestamp`: Timestamp of last prune run

**Examples:**
```bash
# Run server with 30-day post retention
BLUESKY_USERNAME=user.bsky.social BLUESKY_APP_PASSWORD=secret \
./cringesweeper server --platforms=bluesky --max-post-age=30d --preserve-pinned --prune-interval=1h

# Run with Mastodon, delete posts before specific date
MASTODON_USERNAME=user MASTODON_ACCESS_TOKEN=token MASTODON_INSTANCE=https://mastodon.social \
./cringesweeper server --platforms=mastodon --before-date=2024-01-01 --prune-interval=2h

# Test mode - show what would be deleted without actually deleting
./cringesweeper server --platforms=bluesky --max-post-age=7d --dry-run --prune-interval=30m
```

**Docker Deployment:**
```bash
# Build the image
docker build -t cringesweeper .

# Run with environment variables
docker run -d \
  -p 8080:8080 \
  -e BLUESKY_USERNAME=user.bsky.social \
  -e BLUESKY_APP_PASSWORD=your-app-password \
  cringesweeper server --platforms=bluesky --max-post-age=30d --preserve-pinned

# Or using env file
docker run -d -p 8080:8080 --env-file .env cringesweeper server --platforms=bluesky --max-post-age=30d
```

**Access:**
- Health check: http://localhost:8080
- Metrics: http://localhost:8080/metrics

## Authentication Setup

### Bluesky Authentication

Bluesky uses app passwords for API access:

1. Run: `./cringesweeper auth --platforms=bluesky`
2. Visit the URL provided in the output: https://bsky.app/settings/app-passwords
3. Create an app password named "CringeSweeper"
4. Enter your username and app password when prompted

**Required Environment Variables:**
```bash
export BLUESKY_USER="your.username.bsky.social"
export BLUESKY_PASSWORD="your-app-password"
```

### Mastodon Authentication

Mastodon uses OAuth2 access tokens:

1. Run: `./cringesweeper auth --platforms=mastodon`
2. Visit the URL provided in the output for your instance (e.g., https://mastodon.social/settings/applications)
3. Click "New Application" and fill in the details:
   - Application name: "CringeSweeper"
   - Redirect URI: `urn:ietf:wg:oauth:2.0:oob`
   - Required scopes: `read`, `write`
4. Copy the access token when prompted

**Required Environment Variables:**
```bash
export MASTODON_USER="username@instance.social"
export MASTODON_INSTANCE="https://your.instance.social"
export MASTODON_ACCESS_TOKEN="your-access-token"
```

## Post Types

CringeSweeper can identify and handle different types of social media posts:

- **Original**: User's own content
- **Repost**: Shared/retweeted content from others
- **Reply**: Responses to other posts
- **Quote**: Quoted posts with added commentary
- **Like**: Favorited posts (if shown in timeline)

## Configuration

### Credential Storage

Credentials can be stored in two ways:

1. **Environment Variables** (recommended for CI/automation)
2. **Config Files** in `~/.config/cringesweeper/` (recommended for personal use)

The auth command can automatically save credentials to config files for persistence.

### Directory Structure
```
~/.config/cringesweeper/
├── bluesky.json
└── mastodon.json
```

## Examples

### Initial Setup and Status Check
```bash
# Set up authentication for your platforms
./cringesweeper auth --platforms=bluesky
./cringesweeper auth --platforms=mastodon

# Check all your saved credentials
./cringesweeper auth --status
```

### Daily Cleanup Routine
```bash
# Check what would be processed (uses saved credentials)
./cringesweeper prune --max-post-age=365d --preserve-pinned --preserve-selflike --dry-run

# MULTI-PLATFORM: Check cleanup across all platforms
./cringesweeper prune --platforms=all --max-post-age=365d --preserve-pinned --preserve-selflike --dry-run

# Gentle cleanup: unlike old posts, unshare old reposts
./cringesweeper prune --max-post-age=365d --unlike-posts --unshare-reposts --preserve-pinned

# Process entire post history for comprehensive cleanup
./cringesweeper prune --continue --max-post-age=365d --preserve-pinned --preserve-selflike --dry-run

# If you want permanent deletion, run the actual deletion
./cringesweeper prune --max-post-age=365d --preserve-pinned --preserve-selflike
```

### Cross-Platform Management
```bash
# MULTI-PLATFORM: View recent posts from all platforms simultaneously
./cringesweeper ls --platforms=all

# MULTI-PLATFORM: Search through entire timeline on all platforms
./cringesweeper ls --platforms=all --continue --max-post-age=1y

# Legacy: View recent posts from multiple platforms (using saved credentials)
./cringesweeper ls --platforms=bluesky
./cringesweeper ls --platforms=mastodon

# MULTI-PLATFORM: Prune old posts from all platforms
./cringesweeper prune --platforms=all --max-post-age=6m --dry-run

# Legacy: Prune old posts from both platforms separately
./cringesweeper prune --platforms=bluesky --max-post-age=6m --dry-run
./cringesweeper prune --platforms=mastodon --max-post-age=6m --dry-run

# MULTI-PLATFORM: Comprehensive cleanup across all platforms
./cringesweeper prune --platforms=all --continue --max-post-age=6m --dry-run
```

### Spring Cleaning
```bash
# MULTI-PLATFORM: Preview gentle cleanup across all platforms
./cringesweeper prune --platforms=all --before-date="2024-01-01" --unlike-posts --unshare-reposts --preserve-pinned --dry-run

# MULTI-PLATFORM: Comprehensive spring cleaning with continuous processing
./cringesweeper prune --platforms=all --continue --before-date="2024-01-01" --preserve-pinned --preserve-selflike --dry-run

# Preview gentle cleanup before 2024 (unlike + unshare instead of delete)
./cringesweeper prune --before-date="2024-01-01" --unlike-posts --unshare-reposts --preserve-pinned --dry-run

# Comprehensive spring cleaning with continuous processing
./cringesweeper prune --continue --before-date="2024-01-01" --preserve-pinned --preserve-selflike --dry-run

# Delete everything before 2024, but keep important posts
./cringesweeper prune --before-date="2024-01-01" --preserve-pinned --preserve-selflike --dry-run

# Unshare old reposts only (gentler than deletion)
./cringesweeper prune --max-post-age=90d --unshare-reposts --dry-run

# Process entire timeline for thorough cleanup
./cringesweeper prune --continue --max-post-age=90d --unshare-reposts --dry-run
```

### Debugging and Troubleshooting
```bash
# Enable debug logging to see all HTTP requests
./cringesweeper --log-level=debug ls username

# Debug authentication issues
./cringesweeper --log-level=debug auth --status

# Debug pruning with detailed HTTP logging
./cringesweeper --log-level=debug prune --dry-run --max-post-age=30d

# Quiet mode for scripts
./cringesweeper --log-level=error prune --max-post-age=30d
```

## Contributing

Contributions are welcome! This tool is designed to be extensible for additional social media platforms.

### Adding New Platforms

1. Implement the `SocialClient` interface in `internal/`
2. Add authentication flow in `cmd/auth.go`
3. Register the platform in `internal/social.go`

## License

[License information here]

## Disclaimer

**Use at your own risk.** This tool performs permanent deletions of your social media posts. Always test with `--dry-run` first, and ensure you have backups of any content you wish to preserve.

The authors are not responsible for any data loss resulting from the use of this tool.