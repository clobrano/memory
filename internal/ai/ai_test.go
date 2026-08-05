package ai

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/clobrano/memory/internal/config"
	"github.com/clobrano/memory/internal/db"

	_ "modernc.org/sqlite"
)

const testNote = "# Topic\n\nThe body of the note.\n"

func openTestCard(t *testing.T, path string) (*sql.DB, int64) {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.RunMigrations(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	id, err := db.UpsertCard(conn, db.Card{
		Path: path, Title: "Hand written", Tag: "#study", FirstIndexed: time.Now(),
	})
	if err != nil {
		t.Fatalf("UpsertCard: %v", err)
	}
	return conn, id
}

// Hand-written questions must be served on the next review without consulting
// the AI, and must only be flagged once the note itself changes.
func TestSetQuestionsServedUntilNoteChanges(t *testing.T) {
	conn, id := openTestCard(t, "/notes/hand.md")

	if err := SetQuestions(conn, id, testNote, "1. What is the topic?"); err != nil {
		t.Fatalf("SetQuestions: %v", err)
	}

	q, _, stored, changed, err := GetCachedQuestions(conn, id, testNote)
	if err != nil {
		t.Fatalf("GetCachedQuestions: %v", err)
	}
	if !stored {
		t.Fatal("hand-written questions were not stored")
	}
	if q != "1. What is the topic?" {
		t.Errorf("questions = %q, want the hand-written set", q)
	}
	if changed {
		t.Error("noteChanged = true for an unmodified note, would warn on a fresh set")
	}

	// Editing the body flags them as possibly outdated, but still serves them:
	// without AI there is nothing to regenerate with.
	q, _, stored, changed, err = GetCachedQuestions(conn, id, "# Topic\n\nA different body.\n")
	if err != nil {
		t.Fatalf("GetCachedQuestions after edit: %v", err)
	}
	if !stored || q == "" {
		t.Fatal("questions should still be served after the note changed")
	}
	if !changed {
		t.Error("noteChanged = false after the note body changed, warning would be missed")
	}
}

// Frontmatter, and the tag line under the title, are excluded from the hash, so
// touching them must not make good questions look outdated.
func TestSetQuestionsIgnoresFrontmatterEdits(t *testing.T) {
	conn, id := openTestCard(t, "/notes/frontmatter.md")

	withTags := "---\nupdated: 2024-01-01\n---\n# Topic\n#study #topic\n\nThe body of the note.\n"
	if err := SetQuestions(conn, id, withTags, "1. Still relevant?"); err != nil {
		t.Fatalf("SetQuestions: %v", err)
	}

	edited := "---\nupdated: 2026-07-25\n---\n# Topic\n#study #topic #extra\n\nThe body of the note.\n"
	_, _, stored, changed, err := GetCachedQuestions(conn, id, edited)
	if err != nil {
		t.Fatalf("GetCachedQuestions: %v", err)
	}
	if !stored {
		t.Fatal("questions were not stored")
	}
	if changed {
		t.Error("noteChanged = true after only frontmatter and tags changed")
	}
}

// The session shows this error to the user and asks them to decide on it, so it
// has to name the binary and repeat what the binary itself complained about.
func TestAskQuestionsReportsWhatWentWrong(t *testing.T) {
	cfg := config.AIConfig{Binary: "sh", Args: []string{"-c", "echo 'model overloaded' >&2; exit 1"}}

	_, _, _, _, err := AskQuestions(cfg, testNote, nil, 0)
	if err == nil {
		t.Fatal("AskQuestions succeeded with a failing binary")
	}
	if !strings.Contains(err.Error(), "model overloaded") {
		t.Errorf("error %q drops the binary's stderr", err)
	}
	if !strings.Contains(err.Error(), "sh") {
		t.Errorf("error %q does not name the binary", err)
	}
}

// A clean exit with nothing to ask is a failure too: silently returning no
// questions would leave the caller to guess.
func TestAskQuestionsRejectsEmptyOutput(t *testing.T) {
	cfg := config.AIConfig{Binary: "sh", Args: []string{"-c", "printf '  \\n'"}}

	q, _, _, _, err := AskQuestions(cfg, testNote, nil, 0)
	if err == nil {
		t.Fatalf("AskQuestions returned questions %q and no error for empty AI output", q)
	}
	if !strings.Contains(err.Error(), "no questions") {
		t.Errorf("error %q does not say the AI returned nothing", err)
	}
}

func TestInvokeWithoutBinary(t *testing.T) {
	if _, err := invoke(config.AIConfig{}, "prompt"); err == nil {
		t.Error("invoke accepted an empty binary")
	}
}

func TestSetQuestionsRejectsEmpty(t *testing.T) {
	conn, id := openTestCard(t, "/notes/empty.md")

	if err := SetQuestions(conn, id, testNote, "   \n\t\n"); err == nil {
		t.Error("SetQuestions accepted whitespace-only questions")
	}
	if err := SetQuestions(nil, 0, testNote, "1. Anything?"); err == nil {
		t.Error("SetQuestions accepted a nil database")
	}
}
