package cmd

import (
	"database/sql"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/clobrano/memory/internal/db"
	"github.com/clobrano/memory/internal/indexer"
	"github.com/clobrano/memory/internal/tui"
)

var studyCmd = &cobra.Command{
	Use:   "study [keywords...]",
	Short: "Start a study session",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Run indexer
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

		cards, err := db.GetDueCards(DB, args)
		if err != nil {
			return fmt.Errorf("get due cards: %w", err)
		}

		allCards, _ := db.ListAllCards(DB)
		streak := computeStreak(DB)

		model := tui.NewModel(DB, Cfg, cards, len(allCards), streak)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func computeStreak(database *sql.DB) int {
	streak, _ := db.ComputeStreak(database)
	return streak
}

func init() {
	rootCmd.AddCommand(studyCmd)
}
