# Changelog

All notable changes to CringeSweeper will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.1] - Unreleased

### Fixed

- Mastodon replies and boosts were not being pruned. The Mastodon API was called
  with `exclude_replies=true`, preventing replies from ever being fetched and
  therefore never pruned. Now `exclude_replies=false` is sent so replies are
  treated like any other post type.
- Resolved ghcr.io 403 error when pushing multi-platform Docker images.

### Added

- `--exclude-replies` flag for the `prune` command. When set, replies are skipped
  during pruning so reply threads are left intact. Works across all platforms
  (Bluesky and Mastodon). Default behaviour is unchanged — replies are pruned.

## [0.3.0] - 2026-03-01

### Added

- Multi-platform Docker image builds (linux/amd64 and linux/arm64).
- GitHub Actions release workflow that builds and publishes images to ghcr.io on
  version tags.

## [0.2.3] - 2026-02-28

### Fixed

- Infinite loops in pagination caused by duplicate cursors or repeated `max_id`
  values are now detected and halted.

## [0.2.2] - 2026-02-28

### Fixed

- Full pagination for liked posts, reposts, and favourites so all matching records
  are fetched rather than only the first page.

### Removed

- Unused helper functions that were superseded by the paginated fetch variants.

## [0.2.1] - 2026-02-28

### Added

- Comprehensive monitoring infrastructure: Prometheus configuration, Alertmanager
  routing, Grafana dashboards, and a `docker-compose.yml` for the full
  observability stack.

### Fixed

- `PrunePosts` now paginates correctly so posts beyond the first 100 are
  processed.

## [0.2.0] - 2026-02-27

### Added

- Multi-platform server mode (`server` command) with concurrent per-platform
  goroutines, independent scheduling, Prometheus metrics at `/metrics`, a
  real-time web dashboard at `/`, and a JSON status API at `/api/status`.

## [0.1.0] - 2026-02-26

### 🚨 BREAKING CHANGES

- **Removed legacy `--platform` flag**: All commands now require the `--platforms` flag instead
  - **Migration**: Replace `--platform=bluesky` with `--platforms=bluesky`
  - **Migration**: Replace `--platform=mastodon` with `--platforms=mastodon`
  - **New capability**: Use `--platforms=bluesky,mastodon` for multi-platform operations
  - **New capability**: Use `--platforms=all` to operate on all supported platforms
  - **Why**: This change enables true multi-platform support and creates a more consistent CLI interface

### Changed

- All commands (`auth`, `ls`, `prune`, `server`) now require the `--platforms` flag
- Error messages updated to reference `--platforms` instead of `--platform`
- Help documentation updated to reflect the new flag structure
- Command validation now requires explicit platform specification

### Added

- Enhanced multi-platform support with required platform specification
- Improved error messages for missing platform flags
- Clearer validation and user guidance

### Removed

- Legacy `--platform` flag and its shorthand `-p`
- Backward compatibility with single-platform flag syntax
- Default platform fallback behavior

## [0.0.2] and earlier

For changes prior to v0.1.0, please refer to the git commit history.
