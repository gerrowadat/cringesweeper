# CringeSweeper 🧹

A command-line tool for managing your social media presence across multiple platforms. View, analyze, and selectively delete posts from Bluesky, Mastodon, and other social networks.

## Features

- **Multi-platform support**: Works with Bluesky and Mastodon (more platforms coming soon)
- **Post viewing**: List and browse your recent posts across platforms
- **Intelligent pruning**: Delete, unlike, or unshare posts based on age, date, and smart criteria
- **Safety first**: Dry-run mode shows what would be deleted before actually deleting
- **Preserve important posts**: Keep pinned posts and self-liked content
- **Cross-platform authentication**: Guided setup for API keys and tokens
- **Post type detection**: Distinguishes between original posts, reposts, replies, and quotes

## Installation

### From Source

```bash
git clone https://github.com/gerrowadat/cringesweeper.git
cd cringesweeper
go build
```

### Prerequisites

- Go 1.24.2 or later
- Access to your social media accounts for authentication

## Quick Start

1. **Set up authentication** for your preferred platform:
   ```bash
   ./cringesweeper auth --platform=bluesky
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

## Commands

### `ls` - List Recent Posts

Display recent posts from a user's timeline.

```bash
./cringesweeper ls [username] [flags]
```

**Flags:**
- `-p, --platform string`: Social media platform (bluesky, mastodon) (default "bluesky")
- `-h, --help`: Help for ls command

**Examples:**
```bash
# List posts using saved credentials (after running auth command)
./cringesweeper ls

# List posts from a specific Bluesky user
./cringesweeper ls user.bsky.social

# List posts from a Mastodon user
./cringesweeper ls --platform=mastodon user@mastodon.social

# Use environment variable for username (fallback method)
export BLUESKY_USER=user.bsky.social
./cringesweeper ls
```

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

Delete, unlike, or unshare posts from your timeline based on age, date, and preservation rules.

Alternative actions include unliking posts or unsharing reposts instead of deleting them,
providing gentler cleanup options.

```bash
./cringesweeper prune [username] [flags]
```

**Flags:**
- `-p, --platform string`: Social media platform (bluesky, mastodon) (default "bluesky")
- `--max-post-age string`: Delete posts older than this (e.g., 30d, 1y, 24h)
- `--before-date string`: Delete posts created before this date (YYYY-MM-DD or MM/DD/YYYY)
- `--preserve-selflike`: Don't delete user's own posts that they have liked
- `--preserve-pinned`: Don't delete pinned posts
- `--unlike-posts`: Unlike posts instead of deleting them
- `--unshare-reposts`: Unshare/unrepost instead of deleting reposts
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

# Unlike posts instead of deleting them
./cringesweeper prune --max-post-age=90d --unlike-posts --dry-run

# Unshare reposts instead of deleting them
./cringesweeper prune --max-post-age=30d --unshare-reposts --dry-run

# Combined approach: unlike liked posts, unshare reposts, delete the rest
./cringesweeper prune --max-post-age=6m --unlike-posts --unshare-reposts --dry-run

# Delete posts before a specific date for specific user
./cringesweeper prune --before-date="2023-01-01" --dry-run user.bsky.social

# Mastodon pruning with multiple criteria
./cringesweeper prune --platform=mastodon --max-post-age=6m --preserve-selflike

# Bluesky pruning (uses 1s default delay for faster processing)
./cringesweeper prune --platform=bluesky --max-post-age=30d --dry-run

# Mastodon pruning (uses 60s default delay for API compliance)
./cringesweeper prune --platform=mastodon --max-post-age=30d --dry-run

# Custom rate limiting to override platform defaults
./cringesweeper prune --platform=bluesky --max-post-age=30d --rate-limit-delay=500ms --dry-run
```

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

Guide you through setting up authentication credentials for social media platforms.

```bash
./cringesweeper auth [flags]
```

**Flags:**
- `-p, --platform string`: Social media platform to authenticate with (bluesky, mastodon, all) (default "bluesky")
- `--status`: Show credential status for all platforms (ignores --platform flag)
- `-h, --help`: Help for auth command

**Examples:**
```bash
# Setup Bluesky authentication
./cringesweeper auth --platform=bluesky

# Setup Mastodon authentication
./cringesweeper auth --platform=mastodon

# Check credential status for all platforms (--platform flag is ignored with --status)
./cringesweeper auth --status
```

## Authentication Setup

### Bluesky Authentication

Bluesky uses app passwords for API access:

1. Run: `./cringesweeper auth --platform=bluesky`
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

1. Run: `./cringesweeper auth --platform=mastodon`
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
./cringesweeper auth --platform=bluesky
./cringesweeper auth --platform=mastodon

# Check all your saved credentials
./cringesweeper auth --status
```

### Daily Cleanup Routine
```bash
# Check what would be processed (uses saved credentials)
./cringesweeper prune --max-post-age=365d --preserve-pinned --preserve-selflike --dry-run

# Gentle cleanup: unlike old posts, unshare old reposts
./cringesweeper prune --max-post-age=365d --unlike-posts --unshare-reposts --preserve-pinned

# If you want permanent deletion, run the actual deletion
./cringesweeper prune --max-post-age=365d --preserve-pinned --preserve-selflike
```

### Cross-Platform Management
```bash
# View recent posts from multiple platforms (using saved credentials)
./cringesweeper ls --platform=bluesky
./cringesweeper ls --platform=mastodon

# Prune old posts from both platforms
./cringesweeper prune --platform=bluesky --max-post-age=6m --dry-run
./cringesweeper prune --platform=mastodon --max-post-age=6m --dry-run
```

### Spring Cleaning
```bash
# Preview gentle cleanup before 2024 (unlike + unshare instead of delete)
./cringesweeper prune --before-date="2024-01-01" --unlike-posts --unshare-reposts --preserve-pinned --dry-run

# Delete everything before 2024, but keep important posts
./cringesweeper prune --before-date="2024-01-01" --preserve-pinned --preserve-selflike --dry-run

# Unshare old reposts only (gentler than deletion)
./cringesweeper prune --max-post-age=90d --unshare-reposts --dry-run
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