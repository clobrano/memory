package cmd

import (
	"strings"
	"testing"
)

// The editor must open on the text that is already stored, otherwise editing an
// existing set would silently start from a blank buffer. "grep -q" fails, and
// so surfaces as an error, when the temp file does not hold the text.
func TestEditInEditorOpensOnStoredText(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "grep -q '1. What is the topic?'")

	if _, err := editInEditor("1. What is the topic?\n"); err != nil {
		t.Fatalf("editor did not open on the stored questions: %v", err)
	}
}

func TestEditInEditorReadsBackEdits(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "printf '1. Edited?\\n' >")

	got, err := editInEditor("1. Original?\n")
	if err != nil {
		t.Fatalf("editInEditor: %v", err)
	}
	if strings.TrimSpace(got) != "1. Edited?" {
		t.Errorf("got %q, want the edited text", got)
	}
}

func TestEditInEditorPrefersVisual(t *testing.T) {
	t.Setenv("VISUAL", "printf 'from VISUAL\\n' >")
	t.Setenv("EDITOR", "printf 'from EDITOR\\n' >")

	got, err := editInEditor("")
	if err != nil {
		t.Fatalf("editInEditor: %v", err)
	}
	if strings.TrimSpace(got) != "from VISUAL" {
		t.Errorf("got %q, want VISUAL to win over EDITOR", got)
	}
}

// A failed or aborted editor must not be mistaken for an empty set of
// questions, which would otherwise wipe what is stored.
func TestEditInEditorReportsFailure(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "false")

	if _, err := editInEditor("1. Kept?\n"); err == nil {
		t.Error("editInEditor returned no error when the editor failed")
	}
}
