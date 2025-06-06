package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Setup authentication for social media platforms",
	Long: `Interactive setup of authentication credentials for social media platforms.

Guides you through obtaining the necessary API keys, app passwords, and access 
tokens required for authenticated operations like post deletion. Provides 
step-by-step instructions and URLs for each platform's authentication process.

Supports credential storage both as environment variables and in local config files.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		platform, _ := cmd.Flags().GetString("platform")
		status, _ := cmd.Flags().GetBool("status")

		// Handle status flag - always show all platforms when --status is used
		if status {
			showCredentialStatus("all")
			return
		}

		client, exists := internal.GetClient(platform)
		if !exists {
			fmt.Printf("Error: Unsupported platform '%s'. Supported platforms: bluesky, mastodon\n", platform)
			os.Exit(1)
		}

		fmt.Printf("Setting up authentication for %s\n\n", client.GetPlatformName())

		var err error
		switch platform {
		case "bluesky":
			err = setupBlueskyAuth()
		case "mastodon":
			err = setupMastodonAuth()
		default:
			err = fmt.Errorf("authentication not implemented for platform: %s", platform)
		}

		if err != nil {
			fmt.Printf("Error setting up authentication: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ Authentication setup complete for %s!\n", client.GetPlatformName())
		fmt.Println("You can now use commands that require authentication.")
		fmt.Println()
		fmt.Println("💡 Tip: Use 'cringesweeper auth --status' to view your saved credentials.")
	},
}

func setupBlueskyAuth() error {
	fmt.Println("🔐 Bluesky Authentication Setup")
	fmt.Println("===============================")
	fmt.Println()
	fmt.Println("Bluesky uses app passwords for API access.")
	fmt.Println("You'll need to create an app password in your Bluesky settings.")
	fmt.Println()

	fmt.Println("Steps to create an app password:")
	fmt.Println("1. Go to https://bsky.app/settings/app-passwords")
	fmt.Println("2. Log in to your Bluesky account")
	fmt.Println("3. Click 'Add App Password'")
	fmt.Println("4. Give it a name (e.g., 'CringeSweeper')")
	fmt.Println("5. Copy the generated app password")
	fmt.Println()

	// Get username
	fmt.Print("Enter your Bluesky username (e.g., user.bsky.social): ")
	username := strings.TrimSpace(readInput())
	if username == "" {
		return fmt.Errorf("username is required")
	}

	// Get app password
	fmt.Print("Enter your app password: ")
	appPassword := strings.TrimSpace(readInput())
	if appPassword == "" {
		return fmt.Errorf("app password is required")
	}

	// Store credentials
	fmt.Println()
	fmt.Println("Setting environment variables...")
	fmt.Printf("export BLUESKY_USER=\"%s\"\n", username)
	fmt.Printf("export BLUESKY_PASSWORD=\"%s\"\n", appPassword)
	fmt.Println()

	// Optionally save to config file
	fmt.Print("Would you like to save these credentials to ~/.config/cringesweeper? (y/n): ")
	if askYesNo() {
		authManager, err := internal.NewAuthManager()
		if err != nil {
			fmt.Printf("Warning: Could not create auth manager: %v\n", err)
		} else {
			creds := &internal.Credentials{
				Platform:    "bluesky",
				Username:    username,
				AppPassword: appPassword,
			}
			if err := authManager.SaveCredentials(creds); err != nil {
				fmt.Printf("Warning: Could not save credentials: %v\n", err)
			} else {
				fmt.Println("✅ Credentials saved to ~/.config/cringesweeper/bluesky.json")
			}
		}
	}

	fmt.Println("💡 Add the export commands to your shell profile (.bashrc, .zshrc, etc.) to persist them.")

	return nil
}

func setupMastodonAuth() error {
	fmt.Println("🔐 Mastodon Authentication Setup")
	fmt.Println("================================")
	fmt.Println()
	fmt.Println("Mastodon uses OAuth2 for authentication.")
	fmt.Println("You'll need to register an application on your Mastodon instance.")
	fmt.Println()

	// Get instance
	fmt.Print("Enter your Mastodon instance (e.g., mastodon.social): ")
	instance := strings.TrimSpace(readInput())
	if instance == "" {
		return fmt.Errorf("instance is required")
	}

	// Add https:// if not present
	if !strings.HasPrefix(instance, "http") {
		instance = "https://" + instance
	}

	instanceURL := instance
	settingsURL := fmt.Sprintf("%s/settings/applications", instanceURL)

	fmt.Printf("Instance: %s\n", instanceURL)
	fmt.Println()

	fmt.Println("Steps to create an application:")
	fmt.Printf("1. Go to %s\n", settingsURL)
	fmt.Println("2. Log in to your Mastodon account")
	fmt.Println("3. Click 'New Application'")
	fmt.Println("4. Fill in the application details:")
	fmt.Println("   - Application name: CringeSweeper")
	fmt.Println("   - Application website: https://github.com/gerrowadat/cringesweeper")
	fmt.Println("   - Redirect URI: urn:ietf:wg:oauth:2.0:oob")
	fmt.Println("5. Required scopes: read, write")
	fmt.Println("6. Click 'Submit'")
	fmt.Println("7. Copy the access token from the application details")
	fmt.Println()

	// Get username
	fmt.Print("Enter your Mastodon username (without @): ")
	username := strings.TrimSpace(readInput())
	if username == "" {
		return fmt.Errorf("username is required")
	}

	// Get access token
	fmt.Print("Enter your access token: ")
	accessToken := strings.TrimSpace(readInput())
	if accessToken == "" {
		return fmt.Errorf("access token is required")
	}

	// Store credentials
	fmt.Println()
	fmt.Println("Setting environment variables...")
	fullUsername := fmt.Sprintf("%s@%s", username, strings.TrimPrefix(instanceURL, "https://"))
	fmt.Printf("export MASTODON_USER=\"%s\"\n", fullUsername)
	fmt.Printf("export MASTODON_INSTANCE=\"%s\"\n", instanceURL)
	fmt.Printf("export MASTODON_ACCESS_TOKEN=\"%s\"\n", accessToken)
	fmt.Println()

	// Optionally save to config file
	fmt.Print("Would you like to save these credentials to ~/.config/cringesweeper? (y/n): ")
	if askYesNo() {
		authManager, err := internal.NewAuthManager()
		if err != nil {
			fmt.Printf("Warning: Could not create auth manager: %v\n", err)
		} else {
			creds := &internal.Credentials{
				Platform:    "mastodon",
				Username:    fullUsername,
				Instance:    instanceURL,
				AccessToken: accessToken,
			}
			if err := authManager.SaveCredentials(creds); err != nil {
				fmt.Printf("Warning: Could not save credentials: %v\n", err)
			} else {
				fmt.Println("✅ Credentials saved to ~/.config/cringesweeper/mastodon.json")
			}
		}
	}

	fmt.Println("💡 Add the export commands to your shell profile (.bashrc, .zshrc, etc.) to persist them.")

	return nil
}

func askYesNo() bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		input = strings.ToLower(strings.TrimSpace(input))
		switch input {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Print("Please enter 'y' or 'n': ")
		}
	}
}

func readInput() string {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

func showCredentialStatus(platform string) {
	if platform != "all" {
		// Show status for specific platform
		showPlatformStatus(platform)
		return
	}

	// Show status for all supported platforms
	fmt.Println("📋 Credential Status Summary")
	fmt.Println("============================")
	fmt.Println()

	// Get all supported platforms from the internal registry
	supportedPlatforms := []string{"bluesky", "mastodon"}

	for i, p := range supportedPlatforms {
		if i > 0 {
			fmt.Println()
		}
		showPlatformStatus(p)
	}
}

func showPlatformStatus(platform string) {
	fmt.Printf("Platform: %s\n", platform)
	fmt.Printf("─────────%s\n", strings.Repeat("─", len(platform)))

	// Check saved credentials
	authManager, err := internal.NewAuthManager()
	if err != nil {
		fmt.Printf("❌ Error accessing credential storage: %v\n", err)
		return
	}

	creds, err := authManager.LoadCredentials(platform)
	if err != nil {
		fmt.Printf("❌ No saved credentials found\n")
	} else {
		fmt.Printf("✅ Saved credentials found\n")
		fmt.Printf("   Username: %s\n", creds.Username)
		if creds.Instance != "" {
			fmt.Printf("   Instance: %s\n", creds.Instance)
		}

		// Validate credentials
		if err := internal.ValidateCredentials(creds); err != nil {
			fmt.Printf("⚠️  Credentials incomplete: %v\n", err)
		} else {
			fmt.Printf("✅ Credentials complete and valid\n")
		}
	}

	// Check environment variables
	envCreds := internal.GetCredentialsFromEnv(platform)
	if envCreds != nil {
		fmt.Printf("✅ Environment variables found\n")
		if err := internal.ValidateCredentials(envCreds); err != nil {
			fmt.Printf("⚠️  Environment credentials incomplete: %v\n", err)
		}
	} else {
		fmt.Printf("❌ No environment variables found\n")
	}

	// Show what credentials would be used
	finalCreds, err := internal.GetCredentialsForPlatform(platform)
	if err != nil {
		fmt.Printf("❌ No usable credentials available\n")
		fmt.Printf("   Run 'cringesweeper auth --platform=%s' to set up authentication\n", platform)
	} else {
		fmt.Printf("🎯 Active credentials: %s\n", finalCreds.Username)
	}
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.Flags().StringP("platform", "p", "bluesky", "Social media platform to authenticate with (bluesky, mastodon, all)")
	authCmd.Flags().Bool("status", false, "Show credential status instead of setting up authentication")
}
