package cmd

import (
	"fmt"

	"github.com/gerrowadat/cringesweeper/internal"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, commit, and build information for CringeSweeper.`,
	Run: func(cmd *cobra.Command, args []string) {
		versionInfo := internal.GetFullVersionInfo()
		
		fmt.Printf("CringeSweeper %s\n", versionInfo["version"])
		fmt.Printf("Commit: %s\n", versionInfo["commit"])
		fmt.Printf("Build Time: %s\n", versionInfo["build_time"])
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}