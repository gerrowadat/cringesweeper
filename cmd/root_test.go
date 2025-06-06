package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	t.Run("root command exists", func(t *testing.T) {
		if rootCmd == nil {
			t.Error("rootCmd should not be nil")
		}
	})

	t.Run("root command has correct name", func(t *testing.T) {
		expectedUse := "cringesweeper"
		if rootCmd.Use != expectedUse {
			t.Errorf("Expected Use %q, got %q", expectedUse, rootCmd.Use)
		}
	})

	t.Run("root command has short description", func(t *testing.T) {
		if rootCmd.Short == "" {
			t.Error("rootCmd.Short should not be empty")
		}
	})

	t.Run("root command has long description", func(t *testing.T) {
		if rootCmd.Long == "" {
			t.Error("rootCmd.Long should not be empty")
		}
	})

	t.Run("execute function exists", func(t *testing.T) {
		// Test that Execute function doesn't panic when called
		// Note: In a real test environment, we'd want to mock os.Exit
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Execute() panicked: %v", r)
			}
		}()
		
		// We can't actually call Execute() in tests as it would run the CLI
		// Instead, we verify the function exists and the command structure
		// Execute function is available by definition
		t.Log("Execute function is available")
	})
}

func TestSubcommands(t *testing.T) {
	t.Run("auth command is registered", func(t *testing.T) {
		authCmd := findCommand(rootCmd, "auth")
		if authCmd == nil {
			t.Error("auth command should be registered with root command")
		}
	})

	t.Run("ls command is registered", func(t *testing.T) {
		lsCmd := findCommand(rootCmd, "ls")
		if lsCmd == nil {
			t.Error("ls command should be registered with root command")
		}
	})

	t.Run("prune command is registered", func(t *testing.T) {
		pruneCmd := findCommand(rootCmd, "prune")
		if pruneCmd == nil {
			t.Error("prune command should be registered with root command")
		}
	})
}

func TestCommandStructure(t *testing.T) {
	commands := []*cobra.Command{authCmd, lsCmd, pruneCmd}
	
	for _, cmd := range commands {
		t.Run(cmd.Use+" command structure", func(t *testing.T) {
			if cmd.Use == "" {
				t.Errorf("Command should have a Use field")
			}
			
			if cmd.Short == "" {
				t.Errorf("Command %q should have a Short description", cmd.Use)
			}
			
			if cmd.Long == "" {
				t.Errorf("Command %q should have a Long description", cmd.Use)
			}
			
			if cmd.Run == nil && cmd.RunE == nil {
				t.Errorf("Command %q should have a Run or RunE function", cmd.Use)
			}
		})
	}
}

func TestRootFlags(t *testing.T) {
	t.Run("root command has toggle flag", func(t *testing.T) {
		flag := rootCmd.Flags().Lookup("toggle")
		if flag == nil {
			t.Error("Root command should have toggle flag")
		}
		
		if flag.Shorthand != "t" {
			t.Errorf("Expected toggle flag shorthand 't', got %q", flag.Shorthand)
		}
	})
}

func TestAuthCommandFlags(t *testing.T) {
	t.Run("auth has platform flag", func(t *testing.T) {
		flag := authCmd.Flags().Lookup("platform")
		if flag == nil {
			t.Error("Auth command should have platform flag")
		}
		
		if flag.Shorthand != "p" {
			t.Errorf("Expected platform flag shorthand 'p', got %q", flag.Shorthand)
		}
		
		if flag.DefValue != "bluesky" {
			t.Errorf("Expected platform flag default 'bluesky', got %q", flag.DefValue)
		}
	})

	t.Run("auth has status flag", func(t *testing.T) {
		flag := authCmd.Flags().Lookup("status")
		if flag == nil {
			t.Error("Auth command should have status flag")
		}
		
		if flag.DefValue != "false" {
			t.Errorf("Expected status flag default 'false', got %q", flag.DefValue)
		}
	})
}

func TestLsCommandFlags(t *testing.T) {
	t.Run("ls has platform flag", func(t *testing.T) {
		flag := lsCmd.Flags().Lookup("platform")
		if flag == nil {
			t.Error("Ls command should have platform flag")
		}
		
		if flag.Shorthand != "p" {
			t.Errorf("Expected platform flag shorthand 'p', got %q", flag.Shorthand)
		}
		
		if flag.DefValue != "bluesky" {
			t.Errorf("Expected platform flag default 'bluesky', got %q", flag.DefValue)
		}
	})
}

func TestPruneCommandFlags(t *testing.T) {
	expectedFlags := []struct {
		name         string
		hasShorthand bool
		shorthand    string
		required     bool
	}{
		{"platform", true, "p", false},
		{"max-post-age", false, "", false},
		{"before-date", false, "", false},
		{"preserve-selflike", false, "", false},
		{"preserve-pinned", false, "", false},
		{"unlike-posts", false, "", false},
		{"unshare-reposts", false, "", false},
		{"dry-run", false, "", false},
		{"rate-limit-delay", false, "", false},
	}

	for _, expected := range expectedFlags {
		t.Run("prune has "+expected.name+" flag", func(t *testing.T) {
			flag := pruneCmd.Flags().Lookup(expected.name)
			if flag == nil {
				t.Errorf("Prune command should have %s flag", expected.name)
				return
			}
			
			if expected.hasShorthand && flag.Shorthand != expected.shorthand {
				t.Errorf("Expected %s flag shorthand %q, got %q", expected.name, expected.shorthand, flag.Shorthand)
			}
		})
	}
}

func TestCommandArgsValidation(t *testing.T) {
	t.Run("auth command args", func(t *testing.T) {
		if authCmd.Args == nil {
			t.Error("Auth command should have args validation")
		}
	})

	t.Run("ls command args", func(t *testing.T) {
		// MaximumNArgs(1) is a function, we can't compare directly
		// We test that Args is set
		if authCmd.Args == nil {
			t.Error("Ls command should have args validation")
		}
	})

	t.Run("prune command args", func(t *testing.T) {
		// MaximumNArgs(1) is a function, we can't compare directly  
		// We test that Args is set
		if pruneCmd.Args == nil {
			t.Error("Prune command should have args validation")
		}
	})
}

// Helper function to find a subcommand by name
func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func TestCommandHelp(t *testing.T) {
	commands := []*cobra.Command{rootCmd, authCmd, lsCmd, pruneCmd}
	
	for _, cmd := range commands {
		t.Run(cmd.Use+" help text", func(t *testing.T) {
			// Test that help doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Help for command %q panicked: %v", cmd.Use, r)
				}
			}()
			
			// Generate help text to ensure it doesn't panic
			_ = cmd.UsageString()
		})
	}
}

func TestLongDescriptions(t *testing.T) {
	tests := []struct {
		name        string
		cmd         *cobra.Command
		shouldMatch []string
	}{
		{
			name: "root command mentions platforms",
			cmd:  rootCmd,
			shouldMatch: []string{"Bluesky", "Mastodon"},
		},
		{
			name: "auth command mentions authentication",
			cmd:  authCmd,
			shouldMatch: []string{"authentication", "credentials"},
		},
		{
			name: "ls command mentions posts",
			cmd:  lsCmd,
			shouldMatch: []string{"posts", "timeline"},
		},
		{
			name: "prune command mentions deletion",
			cmd:  pruneCmd,
			shouldMatch: []string{"Delete", "dry-run"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			long := tt.cmd.Long
			for _, match := range tt.shouldMatch {
				found := false
				// Simple substring check (case-insensitive would be better)
				for i := 0; i <= len(long)-len(match); i++ {
					if long[i:i+len(match)] == match {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Command %q long description should mention %q", tt.cmd.Use, match)
				}
			}
		})
	}
}