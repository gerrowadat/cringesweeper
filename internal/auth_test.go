package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAuthManager(t *testing.T) {
	authManager, err := NewAuthManager()
	if err != nil {
		t.Fatalf("NewAuthManager() should not return error: %v", err)
	}
	if authManager == nil {
		t.Fatal("NewAuthManager() should return a valid AuthManager")
	}
}

func TestCredentials_Validation(t *testing.T) {
	tests := []struct {
		name        string
		credentials *Credentials
		valid       bool
	}{
		{
			name: "valid bluesky credentials",
			credentials: &Credentials{
				Platform:    "bluesky",
				Username:    "user.bsky.social",
				AppPassword: "test-password",
			},
			valid: true,
		},
		{
			name: "valid mastodon credentials",
			credentials: &Credentials{
				Platform:    "mastodon",
				Username:    "user@mastodon.social",
				Instance:    "https://mastodon.social",
				AccessToken: "test-token",
			},
			valid: true,
		},
		{
			name: "invalid bluesky - missing app password",
			credentials: &Credentials{
				Platform: "bluesky",
				Username: "user.bsky.social",
			},
			valid: false,
		},
		{
			name: "invalid mastodon - missing access token",
			credentials: &Credentials{
				Platform: "mastodon",
				Username: "user@mastodon.social",
				Instance: "https://mastodon.social",
			},
			valid: false,
		},
		{
			name: "invalid mastodon - missing instance",
			credentials: &Credentials{
				Platform:    "mastodon",
				Username:    "user@mastodon.social",
				AccessToken: "test-token",
			},
			valid: false,
		},
		{
			name: "invalid - missing username",
			credentials: &Credentials{
				Platform:    "bluesky",
				AppPassword: "test-password",
			},
			valid: false,
		},
		{
			name: "invalid - unknown platform",
			credentials: &Credentials{
				Platform: "twitter",
				Username: "user",
			},
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateCredentials(test.credentials)
			if test.valid && err != nil {
				t.Errorf("Expected valid credentials but got error: %v", err)
			}
			if !test.valid && err == nil {
				t.Error("Expected invalid credentials but got no error")
			}
		})
	}
}

func TestAuthManager_SaveAndLoadCredentials(t *testing.T) {
	// Create a temporary directory for test
	tempDir := t.TempDir()
	
	// Create auth manager with temporary directory
	authManager := &AuthManager{configDir: tempDir}
	
	testCredentials := &Credentials{
		Platform:    "bluesky",
		Username:    "test.bsky.social",
		AppPassword: "test-password",
	}
	
	// Test saving credentials
	err := authManager.SaveCredentials(testCredentials)
	if err != nil {
		t.Fatalf("SaveCredentials() should not return error: %v", err)
	}
	
	// Verify file was created
	credFile := filepath.Join(tempDir, "bluesky.json")
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		t.Fatal("Credentials file should have been created")
	}
	
	// Test loading credentials
	loadedCredentials, err := authManager.LoadCredentials("bluesky")
	if err != nil {
		t.Fatalf("LoadCredentials() should not return error: %v", err)
	}
	
	// Verify loaded credentials match saved credentials
	if loadedCredentials.Platform != testCredentials.Platform {
		t.Errorf("Platform mismatch: expected %s, got %s", testCredentials.Platform, loadedCredentials.Platform)
	}
	if loadedCredentials.Username != testCredentials.Username {
		t.Errorf("Username mismatch: expected %s, got %s", testCredentials.Username, loadedCredentials.Username)
	}
	if loadedCredentials.AppPassword != testCredentials.AppPassword {
		t.Errorf("AppPassword mismatch: expected %s, got %s", testCredentials.AppPassword, loadedCredentials.AppPassword)
	}
}

func TestAuthManager_LoadCredentials_NotFound(t *testing.T) {
	// Create a temporary directory for test
	tempDir := t.TempDir()
	
	// Create auth manager with temporary directory
	authManager := &AuthManager{configDir: tempDir}
	
	// Test loading non-existent credentials
	_, err := authManager.LoadCredentials("nonexistent")
	if err == nil {
		t.Error("LoadCredentials() should return error for non-existent platform")
	}
}

func TestAuthManager_ListPlatforms(t *testing.T) {
	// Create a temporary directory for test
	tempDir := t.TempDir()
	
	// Create auth manager with temporary directory
	authManager := &AuthManager{configDir: tempDir}
	
	// Initially should be empty
	platforms, err := authManager.ListPlatforms()
	if err != nil {
		t.Fatalf("ListPlatforms() should not return error: %v", err)
	}
	if len(platforms) != 0 {
		t.Errorf("Expected empty platform list, got %d platforms", len(platforms))
	}
	
	// Save some test credentials
	blueskyCredentials := &Credentials{
		Platform:    "bluesky",
		Username:    "test.bsky.social",
		AppPassword: "test-password",
	}
	mastodonCredentials := &Credentials{
		Platform:    "mastodon",
		Username:    "test@mastodon.social",
		Instance:    "https://mastodon.social",
		AccessToken: "test-token",
	}
	
	authManager.SaveCredentials(blueskyCredentials)
	authManager.SaveCredentials(mastodonCredentials)
	
	// Now should have both platforms
	platforms, err = authManager.ListPlatforms()
	if err != nil {
		t.Fatalf("ListPlatentials() should not return error: %v", err)
	}
	if len(platforms) != 2 {
		t.Errorf("Expected 2 platforms, got %d", len(platforms))
	}
	
	// Check that both platforms are present
	platformMap := make(map[string]bool)
	for _, platform := range platforms {
		platformMap[platform] = true
	}
	if !platformMap["bluesky"] {
		t.Error("Expected bluesky platform to be listed")
	}
	if !platformMap["mastodon"] {
		t.Error("Expected mastodon platform to be listed")
	}
}

func TestAuthManager_DeleteCredentials(t *testing.T) {
	// Create a temporary directory for test
	tempDir := t.TempDir()
	
	// Create auth manager with temporary directory
	authManager := &AuthManager{configDir: tempDir}
	
	testCredentials := &Credentials{
		Platform:    "bluesky",
		Username:    "test.bsky.social",
		AppPassword: "test-password",
	}
	
	// Save credentials first
	err := authManager.SaveCredentials(testCredentials)
	if err != nil {
		t.Fatalf("SaveCredentials() should not return error: %v", err)
	}
	
	// Verify credentials exist
	_, err = authManager.LoadCredentials("bluesky")
	if err != nil {
		t.Fatalf("LoadCredentials() should not return error after saving: %v", err)
	}
	
	// Delete credentials
	err = authManager.DeleteCredentials("bluesky")
	if err != nil {
		t.Fatalf("DeleteCredentials() should not return error: %v", err)
	}
	
	// Verify credentials no longer exist
	_, err = authManager.LoadCredentials("bluesky")
	if err == nil {
		t.Error("LoadCredentials() should return error after deletion")
	}
}

func TestCredentials_JSON_Serialization(t *testing.T) {
	originalCredentials := &Credentials{
		Platform:    "mastodon",
		Username:    "test@mastodon.social",
		Instance:    "https://mastodon.social",
		AccessToken: "test-token",
		ExtraData: map[string]string{
			"client_id":     "test-client-id",
			"client_secret": "test-client-secret",
		},
	}
	
	// Test JSON marshaling
	jsonData, err := json.Marshal(originalCredentials)
	if err != nil {
		t.Fatalf("JSON marshaling should not return error: %v", err)
	}
	
	// Test JSON unmarshaling
	var loadedCredentials Credentials
	err = json.Unmarshal(jsonData, &loadedCredentials)
	if err != nil {
		t.Fatalf("JSON unmarshaling should not return error: %v", err)
	}
	
	// Verify all fields match
	if loadedCredentials.Platform != originalCredentials.Platform {
		t.Errorf("Platform mismatch after JSON round-trip")
	}
	if loadedCredentials.Username != originalCredentials.Username {
		t.Errorf("Username mismatch after JSON round-trip")
	}
	if loadedCredentials.Instance != originalCredentials.Instance {
		t.Errorf("Instance mismatch after JSON round-trip")
	}
	if loadedCredentials.AccessToken != originalCredentials.AccessToken {
		t.Errorf("AccessToken mismatch after JSON round-trip")
	}
	
	// Verify ExtraData
	if len(loadedCredentials.ExtraData) != len(originalCredentials.ExtraData) {
		t.Errorf("ExtraData length mismatch after JSON round-trip")
	}
	for key, value := range originalCredentials.ExtraData {
		if loadedCredentials.ExtraData[key] != value {
			t.Errorf("ExtraData[%s] mismatch after JSON round-trip", key)
		}
	}
}