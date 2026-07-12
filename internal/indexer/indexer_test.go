package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTagDetectionFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "note.md", "# My Note\n\nsome content #study here\n")

	notes, err := Scan([]string{dir}, []string{"#study"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes[0].Tag != "#study" {
		t.Errorf("Tag = %q, want #study", notes[0].Tag)
	}
}

func TestTagDetectionNotFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "note.md", "# Untagged\n\nno tags here\n")

	notes, err := Scan([]string{dir}, []string{"#study"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("got %d notes, want 0", len(notes))
	}
}

func TestTitleExtractionH1(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-note.md", "# Great Title\n\n#study\n")

	notes, err := Scan([]string{dir}, []string{"#study"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes[0].Title != "Great Title" {
		t.Errorf("Title = %q, want 'Great Title'", notes[0].Title)
	}
}

func TestTitleFallbackFilename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-note.md", "#study\n\nno heading here\n")

	notes, err := Scan([]string{dir}, []string{"#study"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	if notes[0].Title != "my-note" {
		t.Errorf("Title = %q, want 'my-note'", notes[0].Title)
	}
}

func TestBinaryFileSkipped(t *testing.T) {
	dir := t.TempDir()
	// write a file with a null byte
	path := filepath.Join(dir, "binary.md")
	if err := os.WriteFile(path, []byte("hello\x00world #study"), 0o644); err != nil {
		t.Fatal(err)
	}

	notes, err := Scan([]string{dir}, []string{"#study"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("got %d notes, want 0 (binary skipped)", len(notes))
	}
}

func TestMultipleTags(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", "# A\n#study\n")
	writeFile(t, dir, "b.md", "# B\n#review\n")
	writeFile(t, dir, "c.md", "# C\nno tags\n")

	notes, err := Scan([]string{dir}, []string{"#study", "#review"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Errorf("got %d notes, want 2", len(notes))
	}
}
