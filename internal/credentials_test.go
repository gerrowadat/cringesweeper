package internal

import (
	"os"
	"testing"
)

func TestGetCredentialsFromEnv(t *testing.T) {
	// Save original environment
	originalBlueskyUser := os.Getenv("BLUESKY_USER")
	originalBlueskyPassword := os.Getenv("BLUESKY_PASSWORD")
	originalMastodonUser := os.Getenv("MASTODON_USER")
	originalMastodonInstance := os.Getenv("MASTODON_INSTANCE")
	originalMastodonToken := os.Getenv("MASTODON_ACCESS_TOKEN")
	originalSocialUser := os.Getenv("SOCIAL_USER")
	
	// Clean up environment after test
	defer func() {
		os.Setenv("BLUESKY_USER", originalBlueskyUser)
		os.Setenv("BLUESKY_PASSWORD", originalBlueskyPassword)
		os.Setenv("MASTODON_USER", originalMastodonUser)
		os.Setenv("MASTODON_INSTANCE", originalMastodonInstance)
		os.Setenv("MASTODON_ACCESS_TOKEN", originalMastodonToken)
		os.Setenv("SOCIAL_USER", originalSocialUser)
	}()
	
	tests := []struct {
		name     string
		platform string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "bluesky with complete environment",
			platform: "bluesky",
			envVars: map[string]string{
				"BLUESKY_USER":     "test.bsky.social",
				"BLUESKY_PASSWORD": "test-password",
			},
			expected: true,
		},
		{
			name:     "mastodon with complete environment",
			platform: "mastodon",
			envVars: map[string]string{
				"MASTODON_USER":         "test@mastodon.social",
				"MASTODON_INSTANCE":     "https://mastodon.social",
				"MASTODON_ACCESS_TOKEN": "test-token",
			},
			expected: true,
		},
		{
			name:     "bluesky with fallback to SOCIAL_USER",
			platform: "bluesky",
			envVars: map[string]string{
				"SOCIAL_USER":      "test.bsky.social",
				"BLUESKY_PASSWORD": "test-password",
			},
			expected: true,
		},
		{
			name:     "bluesky missing password",
			platform: "bluesky",
			envVars: map[string]string{
				"BLUESKY_USER": "test.bsky.social",
			},
			expected: false,
		},
		{
			name:     "mastodon missing access token",
			platform: "mastodon",
			envVars: map[string]string{
				"MASTODON_USER":     "test@mastodon.social",
				"MASTODON_INSTANCE": "https://mastodon.social",
			},
			expected: false,
		},
		{
			name:     "unsupported platform",
			platform: "twitter",
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name:     "no environment variables",
			platform: "bluesky",
			envVars:  map[string]string{},
			expected: false,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear all relevant environment variables
			os.Unsetenv("BLUESKY_USER")
			os.Unsetenv("BLUESKY_PASSWORD")
			os.Unsetenv("MASTODON_USER")
			os.Unsetenv("MASTODON_INSTANCE")
			os.Unsetenv("MASTODON_ACCESS_TOKEN")
			os.Unsetenv("SOCIAL_USER")
			
			// Set test environment variables
			for key, value := range test.envVars {
				os.Setenv(key, value)
			}
			
			// Test the function
			creds := GetCredentialsFromEnv(test.platform)
			
			if test.expected && creds == nil {
				t.Error("Expected credentials from environment but got nil")
			}
			if !test.expected && creds != nil {
				t.Error("Expected no credentials from environment but got some")
			}
			
			if creds != nil {
				// Verify platform is set correctly
				if creds.Platform != test.platform {
					t.Errorf("Expected platform %s, got %s", test.platform, creds.Platform)
				}
				
				// Verify credentials are valid
				err := ValidateCredentials(creds)
				if err != nil {
					t.Errorf("Expected valid credentials but validation failed: %v", err)
				}
			}
		})
	}
}

func TestGetUsernameForPlatform(t *testing.T) {
	// Save original environment
	originalBlueskyUser := os.Getenv("BLUESKY_USER")
	originalMastodonUser := os.Getenv("MASTODON_USER")
	originalSocialUser := os.Getenv("SOCIAL_USER")
	
	// Clean up environment after test
	defer func() {
		os.Setenv("BLUESKY_USER", originalBlueskyUser)
		os.Setenv("MASTODON_USER", originalMastodonUser)
		os.Setenv("SOCIAL_USER", originalSocialUser)
	}()
	
	tests := []struct {
		name         string
		platform     string
		argUsername  string
		envVars      map[string]string
		expected     string
		expectError  bool
	}{
		{
			name:        "argument username takes priority",
			platform:    "bluesky",
			argUsername: "arg.bsky.social",
			envVars: map[string]string{
				"BLUESKY_USER": "env.bsky.social",
				"SOCIAL_USER":  "social.bsky.social",
			},
			expected:    "arg.bsky.social",
			expectError: false,
		},
		{
			name:        "fallback to platform-specific env var",
			platform:    "bluesky",
			argUsername: "",
			envVars: map[string]string{
				"BLUESKY_USER": "env.bsky.social",
				"SOCIAL_USER":  "social.bsky.social",
			},
			expected:    "env.bsky.social",
			expectError: false,
		},
		{
			name:        "fallback to generic SOCIAL_USER",
			platform:    "bluesky",
			argUsername: "",
			envVars: map[string]string{
				"SOCIAL_USER": "social.bsky.social",
			},
			expected:    "social.bsky.social",
			expectError: false,
		},
		{
			name:        "no username found",
			platform:    "bluesky",
			argUsername: "",
			envVars:     map[string]string{},
			expected:    "",
			expectError: true,
		},
		{
			name:        "mastodon username from env",
			platform:    "mastodon",
			argUsername: "",
			envVars: map[string]string{
				"MASTODON_USER": "test@mastodon.social",
			},
			expected:    "test@mastodon.social",
			expectError: false,
		},
		{
			name:        "unsupported platform",
			platform:    "twitter",
			argUsername: "",
			envVars:     map[string]string{},
			expected:    "",
			expectError: true,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear environment variables
			os.Unsetenv("BLUESKY_USER")
			os.Unsetenv("MASTODON_USER")
			os.Unsetenv("SOCIAL_USER")
			
			// Set test environment variables
			for key, value := range test.envVars {
				os.Setenv(key, value)
			}
			
			// Test the function
			username, err := GetUsernameForPlatform(test.platform, test.argUsername)
			
			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if username != test.expected {
				t.Errorf("Expected username %s, got %s", test.expected, username)
			}
		})
	}
}

func TestGetCredentialsForPlatform_Priority(t *testing.T) {
	// This test requires temporary directory and environment setup
	tempDir := t.TempDir()
	
	// Save original environment
	originalBlueskyUser := os.Getenv("BLUESKY_USER")
	originalBlueskyPassword := os.Getenv("BLUESKY_PASSWORD")
	
	// Clean up environment after test
	defer func() {
		os.Setenv("BLUESKY_USER", originalBlueskyUser)
		os.Setenv("BLUESKY_PASSWORD", originalBlueskyPassword)
	}()
	
	// Create auth manager with temporary directory
	authManager := &AuthManager{configDir: tempDir}
	
	// Save test credentials to file
	savedCredentials := &Credentials{
		Platform:    "bluesky",
		Username:    "saved.bsky.social",
		AppPassword: "saved-password",
	}
	err := authManager.SaveCredentials(savedCredentials)
	if err != nil {
		t.Fatalf("Failed to save test credentials: %v", err)
	}
	
	// Set environment variables
	os.Setenv("BLUESKY_USER", "env.bsky.social")
	os.Setenv("BLUESKY_PASSWORD", "env-password")
	
	// Test that saved credentials take priority over environment
	creds, err := GetCredentialsForPlatform("bluesky")
	if err != nil {
		t.Fatalf("GetCredentialsForPlatform should not return error: %v", err)
	}
	
	if creds.Username != "saved.bsky.social" {
		t.Errorf("Expected saved credentials to take priority, got username: %s", creds.Username)
	}
	if creds.AppPassword != "saved-password" {
		t.Errorf("Expected saved password, got: %s", creds.AppPassword)
	}
	
	// Delete saved credentials to test environment fallback
	err = authManager.DeleteCredentials("bluesky")
	if err != nil {
		t.Fatalf("Failed to delete test credentials: %v", err)
	}
	
	// Now should fall back to environment
	creds, err = GetCredentialsForPlatform("bluesky")
	if err != nil {
		t.Fatalf("GetCredentialsForPlatform should fall back to environment: %v", err)
	}
	
	if creds.Username != "env.bsky.social" {
		t.Errorf("Expected environment credentials as fallback, got username: %s", creds.Username)
	}
	if creds.AppPassword != "env-password" {
		t.Errorf("Expected environment password, got: %s", creds.AppPassword)
	}
}

func TestGetCredentialsForPlatform_NoCredentials(t *testing.T) {
	// Save original environment
	originalBlueskyUser := os.Getenv("BLUESKY_USER")
	originalBlueskyPassword := os.Getenv("BLUESKY_PASSWORD")
	
	// Clean up environment after test
	defer func() {
		os.Setenv("BLUESKY_USER", originalBlueskyUser)
		os.Setenv("BLUESKY_PASSWORD", originalBlueskyPassword)
	}()
	
	// Clear environment
	os.Unsetenv("BLUESKY_USER")
	os.Unsetenv("BLUESKY_PASSWORD")
	
	// Test with no saved credentials and no environment
	_, err := GetCredentialsForPlatform("bluesky")
	if err == nil {
		t.Error("Expected error when no credentials are available")
	}
}