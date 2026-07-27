package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
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

		return storeQuestions(card, string(raw))
	},
}

var questionsEditCmd = &cobra.Command{
	Use:   "edit <note>",
	Short: "Edit the questions for a note in your editor",
	Long: `Edit the questions for a note in $VISUAL, or $EDITOR, falling back to vi.

The editor opens on whatever is stored today, so an existing set can be
adjusted instead of retyped. Saving stamps the questions with the note's
current content; leaving the buffer unchanged, or empty, stores nothing.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		card, err := resolveCard(args[0])
		if err != nil {
			return err
		}

		// Edit the stored text as-is, so an AI note suggestion below the "---"
		// separator survives a round trip through the editor.
		before := card.CachedQuestions
		after, err := editInEditor(before)
		if err != nil {
			return err
		}

		switch {
		case strings.TrimSpace(after) == "":
			fmt.Println("No questions written, nothing stored.")
			return nil
		case strings.TrimSpace(after) == strings.TrimSpace(before):
			// Storing would re-stamp the note hash, silently clearing any
			// outdated-note warning without the questions having been touched.
			fmt.Println("Questions unchanged, nothing stored.")
			return nil
		}
		return storeQuestions(card, after)
	},
}

// editInEditor round-trips text through the user's editor.
func editInEditor(content string) (string, error) {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "memory-questions-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := f.Name()
	defer os.Remove(path)

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	// Run through a shell so an editor setting that carries arguments keeps
	// working, such as EDITOR="code --wait".
	editorCmd := exec.Command("sh", "-c", editor+` "`+path+`"`)
	editorCmd.Stdin, editorCmd.Stdout, editorCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := editorCmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q: %w", editor, err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read back questions: %w", err)
	}
	return string(edited), nil
}

// storeQuestions writes questions for a card, stamped with the note as it
// stands right now.
func storeQuestions(card *db.Card, questions string) error {
	content, err := os.ReadFile(card.Path)
	if err != nil {
		return fmt.Errorf("read %s: %w", card.Path, err)
	}
	if err := ai.SetQuestions(DB, card.ID, string(content), questions); err != nil {
		return err
	}
	fmt.Printf("Stored questions for %q.\n", card.Title)
	return nil
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
	questionsCmd.AddCommand(questionsEditCmd)
	rootCmd.AddCommand(questionsCmd)
}
