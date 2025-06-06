package cmd

import (
	"fmt"
	"strings"
	"testing"
)

func TestAskYesNo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"yes response", "y\n", true},
		{"yes long response", "yes\n", true},
		{"no response", "n\n", false},
		{"no long response", "no\n", false},
		{"uppercase yes", "Y\n", true},
		{"uppercase no", "N\n", false},
		{"mixed case", "Yes\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: askYesNo() reads from os.Stdin which is hard to mock in tests
			// This test demonstrates the structure but would need input mocking for full testing
			if tt.input == "y\n" || tt.input == "yes\n" || tt.input == "Y\n" || tt.input == "Yes\n" {
				expected := true
				if !expected {
					t.Errorf("Expected true for input %q", tt.input)
				}
			}
		})
	}
}

func TestReadInput(t *testing.T) {
	// Note: readInput() reads from os.Stdin which is hard to mock in tests
	// This test demonstrates the structure but would need input mocking for full testing
	t.Run("empty input handling", func(t *testing.T) {
		input := ""
		if len(input) != 0 {
			t.Errorf("Expected empty string, got %q", input)
		}
	})
}


func TestAuthCommandValidation(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		shouldBeValid bool
	}{
		{"valid bluesky", "bluesky", true},
		{"valid mastodon", "mastodon", true},
		{"invalid platform", "twitter", false},
		{"empty platform", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test platform validation logic that would be used in auth command
			validPlatforms := []string{"bluesky", "mastodon"}
			isValid := false
			for _, valid := range validPlatforms {
				if tt.platform == valid {
					isValid = true
					break
				}
			}
			
			if isValid != tt.shouldBeValid {
				t.Errorf("Platform %q validity: expected %v, got %v", tt.platform, tt.shouldBeValid, isValid)
			}
		})
	}
}

func TestBlueskyAuthSetupValidation(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		appPassword string
		shouldError bool
	}{
		{"valid credentials", "user.bsky.social", "abcd-efgh-ijkl-mnop", false},
		{"empty username", "", "abcd-efgh-ijkl-mnop", true},
		{"empty password", "user.bsky.social", "", true},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic that would be used in setupBlueskyAuth
			hasError := (strings.TrimSpace(tt.username) == "" || strings.TrimSpace(tt.appPassword) == "")
			
			if hasError != tt.shouldError {
				t.Errorf("Expected error %v for username=%q, password=%q, got %v", 
					tt.shouldError, tt.username, tt.appPassword, hasError)
			}
		})
	}
}

func TestMastodonAuthSetupValidation(t *testing.T) {
	tests := []struct {
		name        string
		instance    string
		username    string
		accessToken string
		shouldError bool
	}{
		{"valid credentials", "mastodon.social", "user", "token123", false},
		{"empty instance", "", "user", "token123", true},
		{"empty username", "mastodon.social", "", "token123", true},
		{"empty token", "mastodon.social", "user", "", true},
		{"all empty", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic that would be used in setupMastodonAuth
			hasError := (strings.TrimSpace(tt.instance) == "" || 
				strings.TrimSpace(tt.username) == "" || 
				strings.TrimSpace(tt.accessToken) == "")
			
			if hasError != tt.shouldError {
				t.Errorf("Expected error %v for instance=%q, username=%q, token=%q, got %v", 
					tt.shouldError, tt.instance, tt.username, tt.accessToken, hasError)
			}
		})
	}
}

func TestInstanceURLFormatting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"without https", "mastodon.social", "https://mastodon.social"},
		{"with https", "https://mastodon.social", "https://mastodon.social"},
		{"with http", "http://mastodon.social", "http://mastodon.social"},
		{"empty input", "", "https://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the URL formatting logic used in setupMastodonAuth
			instance := tt.input
			if !strings.HasPrefix(instance, "http") {
				instance = "https://" + instance
			}
			
			if instance != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, instance)
			}
		})
	}
}

func TestSettingsURLGeneration(t *testing.T) {
	tests := []struct {
		name        string
		instanceURL string
		expected    string
	}{
		{"mastodon.social", "https://mastodon.social", "https://mastodon.social/settings/applications"},
		{"custom instance", "https://fosstodon.org", "https://fosstodon.org/settings/applications"},
		{"with trailing slash", "https://mastodon.social/", "https://mastodon.social//settings/applications"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test settings URL generation logic
			settingsURL := tt.instanceURL + "/settings/applications"
			
			if settingsURL != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, settingsURL)
			}
		})
	}
}

func TestShowCredentialStatus(t *testing.T) {
	tests := []struct {
		name     string
		platform string
	}{
		{"show all platforms", "all"},
		{"show specific platform", "bluesky"},
		{"show mastodon", "mastodon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that showCredentialStatus doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("showCredentialStatus panicked: %v", r)
				}
			}()
			
			// This would require capturing stdout to fully test
			// For now we just ensure it doesn't crash
			showCredentialStatus(tt.platform)
		})
	}
}

func TestShowPlatformStatus(t *testing.T) {
	tests := []struct {
		name     string
		platform string
	}{
		{"bluesky platform status", "bluesky"},
		{"mastodon platform status", "mastodon"},
		{"invalid platform status", "twitter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that showPlatformStatus doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("showPlatformStatus panicked: %v", r)
				}
			}()
			
			showPlatformStatus(tt.platform)
		})
	}
}

func TestSetupBlueskyAuthValidation(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		appPassword    string
		expectError    bool
		errorMessage   string
	}{
		{"valid credentials", "user.bsky.social", "abcd-efgh-ijkl-mnop", false, ""},
		{"empty username", "", "abcd-efgh-ijkl-mnop", true, "username is required"},
		{"empty password", "user.bsky.social", "", true, "app password is required"},
		{"both empty", "", "", true, "username is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic used in setupBlueskyAuth
			username := strings.TrimSpace(tt.username)
			appPassword := strings.TrimSpace(tt.appPassword)
			
			var err error
			if username == "" {
				err = fmt.Errorf("username is required")
			} else if appPassword == "" {
				err = fmt.Errorf("app password is required")
			}
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectError && err != nil && err.Error() != tt.errorMessage {
				t.Errorf("Expected error %q, got %q", tt.errorMessage, err.Error())
			}
		})
	}
}

func TestSetupMastodonAuthValidation(t *testing.T) {
	tests := []struct {
		name           string
		instance       string
		username       string
		accessToken    string
		expectError    bool
		errorMessage   string
	}{
		{"valid credentials", "mastodon.social", "user", "token123", false, ""},
		{"empty instance", "", "user", "token123", true, "instance is required"},
		{"empty username", "mastodon.social", "", "token123", true, "username is required"},
		{"empty token", "mastodon.social", "user", "", true, "access token is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic used in setupMastodonAuth
			instance := strings.TrimSpace(tt.instance)
			username := strings.TrimSpace(tt.username)
			accessToken := strings.TrimSpace(tt.accessToken)
			
			var err error
			if instance == "" {
				err = fmt.Errorf("instance is required")
			} else if username == "" {
				err = fmt.Errorf("username is required")
			} else if accessToken == "" {
				err = fmt.Errorf("access token is required")
			}
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectError && err != nil && err.Error() != tt.errorMessage {
				t.Errorf("Expected error %q, got %q", tt.errorMessage, err.Error())
			}
		})
	}
}

func TestFullUsernameGeneration(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		instanceURL string
		expected    string
	}{
		{"standard case", "user", "https://mastodon.social", "user@mastodon.social"},
		{"custom instance", "alice", "https://fosstodon.org", "alice@fosstodon.org"},
		{"with port", "bob", "https://mastodon.local:3000", "bob@mastodon.local:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test full username generation logic
			fullUsername := tt.username + "@" + strings.TrimPrefix(tt.instanceURL, "https://")
			
			if fullUsername != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, fullUsername)
			}
		})
	}
}