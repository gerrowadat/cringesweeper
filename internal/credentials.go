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

// GetCredentialsForPlatformEnvOnly only loads credentials from environment variables (for server mode)
func GetCredentialsForPlatformEnvOnly(platform string) (*Credentials, error) {
	creds := GetCredentialsFromEnv(platform)
	if creds == nil {
		return nil, fmt.Errorf("no credentials found in environment variables for platform %s. In server mode, credentials must be provided via environment variables", platform)
	}
	
	if err := ValidateCredentials(creds); err != nil {
		return nil, fmt.Errorf("invalid credentials from environment variables for platform %s: %w", platform, err)
	}
	
	return creds, nil
}

// GetUsernameForPlatformEnvOnly gets username from environment variables only (for server mode)
func GetUsernameForPlatformEnvOnly(platform string, argUsername string) (string, error) {
	// If username provided as argument, use it
	if argUsername != "" {
		return argUsername, nil
	}

	// Only try environment variables (no saved credentials in server mode)
	switch platform {
	case "bluesky":
		if username := os.Getenv("BLUESKY_USER"); username != "" {
			return username, nil
		}
		if username := os.Getenv("BLUESKY_USERNAME"); username != "" {
			return username, nil
		}
	case "mastodon":
		if username := os.Getenv("MASTODON_USER"); username != "" {
			return username, nil
		}
		if username := os.Getenv("MASTODON_USERNAME"); username != "" {
			return username, nil
		}
	}

	// Final fallback to generic environment variable
	if username := os.Getenv("SOCIAL_USER"); username != "" {
		return username, nil
	}

	return "", fmt.Errorf("no username found in environment variables. In server mode, please provide a username as an argument or set %s_USERNAME environment variable", map[string]string{"bluesky": "BLUESKY", "mastodon": "MASTODON"}[platform])
}
