/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/spf13/cobra"
)

var logLevel string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cringesweeper",
	Short: "Cross-platform social media post management tool",
	Long: `CringeSweeper is a command-line tool for managing your social media presence 
across multiple platforms including Bluesky and Mastodon.

Key features:
• List and browse recent posts from your timelines
• Delete posts based on age, date, and smart preservation rules  
• Safe dry-run mode to preview deletions before executing
• Cross-platform authentication setup and management
• Support for multiple post types (original, reposts, replies, quotes)

Use 'cringesweeper [command] --help' for detailed information about each command.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger with the specified log level before any command runs
		internal.InitLoggerWithLevel(logLevel)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cringesweeper.yaml)")

	// Add log level flag that applies to all commands
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set the logging level (debug, info, warn, error)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
