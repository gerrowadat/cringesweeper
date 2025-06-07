package internal

import (
	"runtime/debug"
	"strings"
)

// Version information
var (
	// Version is set by build-time flags or git tags
	Version = "dev"
	// Commit is set by build-time flags  
	Commit = "unknown"
	// BuildTime is set by build-time flags
	BuildTime = "unknown"
)

// GetVersion returns the current version information
func GetVersion() string {
	if Version != "dev" {
		return Version
	}
	
	// Try to get version from build info (when built with go install)
	if info, ok := debug.ReadBuildInfo(); ok {
		// Look for version in build info
		for _, setting := range info.Settings {
			if setting.Key == "vcs.tag" && setting.Value != "" {
				// Clean up git tag (remove 'v' prefix if present)
				version := strings.TrimPrefix(setting.Value, "v")
				if version != "" {
					return version
				}
			}
		}
		
		// Fallback to revision if available
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				return "dev-" + setting.Value[:8]
			}
		}
	}
	
	return "dev"
}

// GetFullVersionInfo returns detailed version information
func GetFullVersionInfo() map[string]string {
	version := GetVersion()
	commit := Commit
	buildTime := BuildTime
	
	// Try to get additional info from build info
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == "unknown" && setting.Value != "" {
					commit = setting.Value[:8]
				}
			case "vcs.time":
				if buildTime == "unknown" && setting.Value != "" {
					buildTime = setting.Value
				}
			}
		}
	}
	
	return map[string]string{
		"version":    version,
		"commit":     commit,
		"build_time": buildTime,
	}
}