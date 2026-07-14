package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/clobrano/memory/internal/ai"
)

var promptsCmd = &cobra.Command{
	Use:   "prompts",
	Short: "Manage AI prompt templates",
	// Override root PersistentPreRunE — no config validation needed here.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
}

var promptsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset AI prompt templates to the built-in defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		return ai.ResetPrompts(filepath.Dir(cfgFile))
	},
}

func init() {
	promptsCmd.AddCommand(promptsResetCmd)
	rootCmd.AddCommand(promptsCmd)
}
