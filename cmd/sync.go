package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/clobrano/memory/internal/db"
	"github.com/clobrano/memory/internal/indexer"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync vault notes with the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		scanned, err := indexer.Scan(Cfg.NotesDirs, Cfg.StudyTags)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		stored, err := db.ListAllCards(DB)
		if err != nil {
			return fmt.Errorf("list cards: %w", err)
		}

		newNotes, missing := indexer.Diff(scanned, stored)

		for _, n := range newNotes {
			_, err := db.UpsertCard(DB, db.Card{
				Path:         n.Path,
				Title:        n.Title,
				Tag:          n.Tag,
				FirstIndexed: time.Now(),
			})
			if err != nil {
				fmt.Printf("warn: upsert %s: %v\n", n.Path, err)
			}
		}

		if err := indexer.ResolveMissing(DB, missing); err != nil {
			return err
		}

		unchanged := len(stored) - len(missing)
		fmt.Printf("Sync complete: +%d added, -%d removed, %d unchanged\n",
			len(newNotes), len(missing), unchanged)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
