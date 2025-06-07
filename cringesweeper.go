package main

import (
	"github.com/gerrowadat/cringesweeper/cmd"
)

func main() {
	// Logger will be initialized by the root command's PersistentPreRun
	// which runs before any subcommand and respects the --log-level flag
	cmd.Execute()
}
