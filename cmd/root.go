package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clobrano/memory/internal/config"
	"github.com/clobrano/memory/internal/db"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dbFile  string
	Cfg     *config.Config
	DB      *sql.DB
)

var rootCmd = &cobra.Command{
	Use:   "memory",
	Short: "Spaced-repetition study tool for Obsidian-style markdown vaults",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		Cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}
		if err := config.Validate(Cfg); err != nil {
			return fmt.Errorf("config invalid: %w", err)
		}
		DB, err = db.Open(dbFile)
		if err != nil {
			return fmt.Errorf("db: %w", err)
		}
		return db.RunMigrations(DB)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	home, _ := os.UserHomeDir()
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config",
		filepath.Join(home, ".config", "memory", "config.toml"), "config file path")
	rootCmd.PersistentFlags().StringVar(&dbFile, "db",
		filepath.Join(home, ".local", "share", "memory", "db.sqlite"), "database file path")
}
