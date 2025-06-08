package internal

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	version := GetVersion()
	
	// Version should not be empty
	if version == "" {
		t.Error("GetVersion() should not return empty string")
	}
	
	// Should return either a version or "dev"
	if version != "dev" && !strings.Contains(version, ".") {
		t.Errorf("GetVersion() returned unexpected format: %q", version)
	}
}

func TestGetFullVersionInfo(t *testing.T) {
	info := GetFullVersionInfo()
	
	// Should contain expected keys
	expectedKeys := []string{"version", "commit", "build_time"}
	for _, key := range expectedKeys {
		if _, exists := info[key]; !exists {
			t.Errorf("GetFullVersionInfo() missing key: %q", key)
		}
	}
	
	// Values should not be empty
	for key, value := range info {
		if value == "" {
			t.Errorf("GetFullVersionInfo() has empty value for key: %q", key)
		}
	}
}

func TestVersionConsistency(t *testing.T) {
	// Version from GetVersion should match version in GetFullVersionInfo
	version := GetVersion()
	fullInfo := GetFullVersionInfo()
	
	if version != fullInfo["version"] {
		t.Errorf("Version mismatch: GetVersion()=%q, GetFullVersionInfo()[\"version\"]=%q", 
			version, fullInfo["version"])
	}
}

func TestBuildTimeFormat(t *testing.T) {
	info := GetFullVersionInfo()
	buildTime := info["build_time"]
	
	// Build time should be in RFC3339 format (YYYY-MM-DDTHH:MM:SSZ)
	if !strings.Contains(buildTime, "T") || !strings.HasSuffix(buildTime, "Z") {
		// Only warn, don't fail, as this depends on build context
		t.Logf("Build time may not be in expected RFC3339 format: %q", buildTime)
	}
}

func TestCommitFormat(t *testing.T) {
	info := GetFullVersionInfo()
	commit := info["commit"]
	
	// Commit should be either "unknown" or a git hash
	if commit != "unknown" && len(commit) < 7 {
		t.Errorf("Commit hash seems too short: %q", commit)
	}
}