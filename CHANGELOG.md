# Changelog

All notable changes to CringeSweeper will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-06-08

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

## [Unreleased]

### Previous Releases

This changelog starts with version 0.1.0. For changes prior to this version, please refer to the git commit history.