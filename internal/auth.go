package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials stores authentication information for a platform
type Credentials struct {
	Platform    string            `json:"platform"`
	Username    string            `json:"username"`
	Instance    string            `json:"instance,omitempty"` // For Mastodon
	AccessToken string            `json:"access_token,omitempty"`
	AppPassword string            `json:"app_password,omitempty"` // For Bluesky
	ExtraData   map[string]string `json:"extra_data,omitempty"`
}

// AuthManager handles credential storage and retrieval
type AuthManager struct {
	configDir string
}

// NewAuthManager creates a new authentication manager
func NewAuthManager() (*AuthManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "cringesweeper")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &AuthManager{configDir: configDir}, nil
}

// SaveCredentials stores credentials for a platform
func (am *AuthManager) SaveCredentials(creds *Credentials) error {
	filename := filepath.Join(am.configDir, fmt.Sprintf("%s.json", creds.Platform))

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// LoadCredentials retrieves stored credentials for a platform
func (am *AuthManager) LoadCredentials(platform string) (*Credentials, error) {
	filename := filepath.Join(am.configDir, fmt.Sprintf("%s.json", platform))

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no credentials found for platform %s", platform)
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &creds, nil
}

// DeleteCredentials removes stored credentials for a platform
func (am *AuthManager) DeleteCredentials(platform string) error {
	filename := filepath.Join(am.configDir, fmt.Sprintf("%s.json", platform))

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials file: %w", err)
	}

	return nil
}

// ListPlatforms returns a list of platforms with stored credentials
func (am *AuthManager) ListPlatforms() ([]string, error) {
	files, err := os.ReadDir(am.configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	var platforms []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			platform := file.Name()[:len(file.Name())-5] // Remove .json extension
			platforms = append(platforms, platform)
		}
	}

	return platforms, nil
}

// GetCredentialsFromEnv retrieves credentials from environment variables
func GetCredentialsFromEnv(platform string) *Credentials {
	switch platform {
	case "bluesky":
		username := os.Getenv("BLUESKY_USER")
		password := os.Getenv("BLUESKY_PASSWORD")
		if username != "" && password != "" {
			return &Credentials{
				Platform:    platform,
				Username:    username,
				AppPassword: password,
			}
		}
	case "mastodon":
		username := os.Getenv("MASTODON_USER")
		instance := os.Getenv("MASTODON_INSTANCE")
		token := os.Getenv("MASTODON_ACCESS_TOKEN")
		if username != "" && instance != "" && token != "" {
			return &Credentials{
				Platform:    platform,
				Username:    username,
				Instance:    instance,
				AccessToken: token,
			}
		}
	}
	return nil
}

// ValidateCredentials checks if credentials are complete for a platform
func ValidateCredentials(creds *Credentials) error {
	if creds == nil {
		return fmt.Errorf("credentials are nil")
	}

	if creds.Username == "" {
		return fmt.Errorf("username is required")
	}

	switch creds.Platform {
	case "bluesky":
		if creds.AppPassword == "" {
			return fmt.Errorf("app password is required for Bluesky")
		}
	case "mastodon":
		if creds.Instance == "" {
			return fmt.Errorf("instance is required for Mastodon")
		}
		if creds.AccessToken == "" {
			return fmt.Errorf("access token is required for Mastodon")
		}
	default:
		return fmt.Errorf("unsupported platform: %s", creds.Platform)
	}

	return nil
}
