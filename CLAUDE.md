# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building
```bash
go build  # Builds the main binary as 'cringesweeper'
```

### Testing
```bash
go test ./...        # Run all tests
go test ./internal   # Run tests for internal package only
```

Note: Some tests currently fail due to environment variable dependencies in the test environment.

### Code Formatting
```bash
gofmt -w .  # Format all Go code
```

## Architecture Overview

CringeSweeper is a CLI tool for managing social media posts across multiple platforms (Bluesky, Mastodon). 

### Core Architecture

The codebase follows a clean separation of concerns:

- **`cmd/`** - Cobra CLI commands (auth, ls, prune, root)
- **`internal/`** - Core business logic and platform implementations
- **`cringesweeper.go`** - Main entry point

### Key Components

1. **SocialClient Interface** (`internal/social.go`): 
   - Defines common operations for all social platforms
   - Implemented by BlueskyClient and MastodonClient
   - Supports post fetching, pruning, and authentication requirements

2. **Post Structure** (`internal/social.go`):
   - Unified post representation across platforms
   - Supports different post types: original, repost, reply, like, quote
   - Includes engagement metrics and platform-specific metadata

3. **Authentication System** (`internal/auth.go`, `internal/credentials.go`):
   - Multi-layered credential resolution: CLI args → saved files → environment variables
   - Platform-specific credential validation
   - Credentials stored in `~/.config/cringesweeper/`

4. **Pruning Engine** (`internal/social.go`):
   - Configurable deletion criteria (age, date, preservation rules)
   - Safety features: dry-run mode, preserve pinned/self-liked posts
   - Alternative actions: unlike posts, unshare reposts instead of deletion

### Platform Implementations

Each social platform implements the `SocialClient` interface:

- **Bluesky** (`internal/bsky.go`): Uses app passwords, faster rate limits (1s default)
- **Mastodon** (`internal/mastodon.go`): Uses OAuth2 tokens, conservative rate limits (60s default)

### Adding New Platforms

To add support for a new social platform:

1. Implement `SocialClient` interface in `internal/`
2. Add authentication flow in `cmd/auth.go` 
3. Register platform in `SupportedPlatforms` map in `internal/social.go`
4. Add credential validation logic in `internal/credentials.go`

### Command Structure

- **`auth`**: Platform authentication setup and credential management
- **`ls`**: List recent posts from user timelines
- **`prune`**: Delete/unlike/unshare posts based on criteria

All commands support `--platform` flag to specify target social network.

### Testing

The test suite includes:
- Unit tests for credential management and validation
- Social client interface compliance tests
- Post type and pruning logic validation

Tests use table-driven patterns and may require environment setup for some credential-related tests.