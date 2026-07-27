package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/clobrano/memory/internal/ai"
	"github.com/clobrano/memory/internal/db"
)

var questionsFile string

var questionsCmd = &cobra.Command{
	Use:   "questions",
	Short: "Show or write the study questions for a note",
}

var questionsShowCmd = &cobra.Command{
	Use:   "show <note>",
	Short: "Show the stored questions for a note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		card, err := resolveCard(args[0])
		if err != nil {
			return err
		}
		content, err := os.ReadFile(card.Path)
		if err != nil {
			return fmt.Errorf("read %s: %w", card.Path, err)
		}

		questions, suggestions, stored, noteChanged, err := ai.GetCachedQuestions(DB, card.ID, string(content))
		if err != nil {
			return err
		}
		if !stored || questions == "" {
			fmt.Printf("No questions stored for %q.\n", card.Title)
			fmt.Println("Write some with 'memory questions set', or study the note with AI enabled.")
			return nil
		}

		fmt.Println(card.Title)
		if noteChanged {
			fmt.Println(yellowStyle.Render("⚠ Note has changed, possible outdated questions"))
		}
		fmt.Printf("\n%s\n", questions)
		if suggestions != "" {
			fmt.Printf("\n--- note suggestion ---\n%s\n", suggestions)
		}
		return nil
	},
}

var questionsSetCmd = &cobra.Command{
	Use:   "set <note>",
	Short: "Write your own questions for a note",
	Long: `Write your own questions for a note, read from --file or from stdin.

The questions are stamped with the note's current content, so the next review
serves them as-is without calling the AI. They replace whatever was stored
before, including questions the AI generated.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		card, err := resolveCard(args[0])
		if err != nil {
			return err
		}

		var raw []byte
		if questionsFile != "" {
			raw, err = os.ReadFile(questionsFile)
			if err != nil {
				return fmt.Errorf("read %s: %w", questionsFile, err)
			}
		} else {
			raw, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
		}

		content, err := os.ReadFile(card.Path)
		if err != nil {
			return fmt.Errorf("read %s: %w", card.Path, err)
		}
		if err := ai.SetQuestions(DB, card.ID, string(content), string(raw)); err != nil {
			return err
		}

		fmt.Printf("Stored questions for %q.\n", card.Title)
		return nil
	},
}

// resolveCard finds the tracked note an argument refers to, accepting either a
// path or a fragment of a path or title.
func resolveCard(arg string) (*db.Card, error) {
	if abs, err := filepath.Abs(arg); err == nil {
		if card, err := db.GetCardByPath(DB, abs); err == nil && card != nil {
			return card, nil
		}
	}

	cards, err := db.ListAllCards(DB)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(arg)
	var matches []db.Card
	for _, c := range cards {
		if strings.Contains(strings.ToLower(c.Path), needle) || strings.Contains(strings.ToLower(c.Title), needle) {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no tracked note matches %q (run 'memory sync' if it is new)", arg)
	case 1:
		return &matches[0], nil
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "%q matches %d notes:", arg, len(matches))
		for _, c := range matches {
			fmt.Fprintf(&b, "\n  %s", c.Path)
		}
		return nil, fmt.Errorf("%s", b.String())
	}
}

func init() {
	questionsSetCmd.Flags().StringVar(&questionsFile, "file", "",
		"read questions from a file instead of stdin")
	questionsCmd.AddCommand(questionsShowCmd)
	questionsCmd.AddCommand(questionsSetCmd)
	rootCmd.AddCommand(questionsCmd)
}
