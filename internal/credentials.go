package internal

import (
	"fmt"
	"os"
)

// GetCredentialsForPlatform attempts to load credentials using multiple fallback methods
func GetCredentialsForPlatform(platform string) (*Credentials, error) {
	// First, try to load from saved config files
	authManager, err := NewAuthManager()
	if err == nil {
		if creds, err := authManager.LoadCredentials(platform); err == nil {
			if err := ValidateCredentials(creds); err == nil {
				return creds, nil
			}
		}
	}

	// Second, try to load from environment variables
	if creds := GetCredentialsFromEnv(platform); creds != nil {
		if err := ValidateCredentials(creds); err == nil {
			return creds, nil
		}
	}

	return nil, fmt.Errorf("no valid credentials found for platform %s. Run 'cringesweeper auth --platform=%s' to set up authentication", platform, platform)
}

// GetUsernameForPlatform gets username with fallback priority: argument > saved credentials > environment
func GetUsernameForPlatform(platform string, argUsername string) (string, error) {
	// If username provided as argument, use it
	if argUsername != "" {
		return argUsername, nil
	}

	// Try to get username from saved credentials
	authManager, err := NewAuthManager()
	if err == nil {
		if creds, err := authManager.LoadCredentials(platform); err == nil {
			if creds.Username != "" {
				return creds.Username, nil
			}
		}
	}

	// Fallback to environment variables
	switch platform {
	case "bluesky":
		if username := os.Getenv("BLUESKY_USER"); username != "" {
			return username, nil
		}
	case "mastodon":
		if username := os.Getenv("MASTODON_USER"); username != "" {
			return username, nil
		}
	}

	// Final fallback to generic environment variable
	if username := os.Getenv("SOCIAL_USER"); username != "" {
		return username, nil
	}

	return "", fmt.Errorf("no username found. Please provide a username as an argument, run 'cringesweeper auth --platform=%s', or set %s_USER environment variable", platform, map[string]string{"bluesky": "BLUESKY", "mastodon": "MASTODON"}[platform])
}