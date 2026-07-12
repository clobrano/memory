package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `notes_dirs = ["/tmp/notes"]
study_tags = ["#review"]
daily_limit = 10
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.NotesDirs) != 1 || cfg.NotesDirs[0] != "/tmp/notes" {
		t.Errorf("NotesDirs = %v, want [/tmp/notes]", cfg.NotesDirs)
	}
	if len(cfg.StudyTags) != 1 || cfg.StudyTags[0] != "#review" {
		t.Errorf("StudyTags = %v, want [#review]", cfg.StudyTags)
	}
	if cfg.DailyLimit != 10 {
		t.Errorf("DailyLimit = %d, want 10", cfg.DailyLimit)
	}
}

func TestLoadMissingFileCreatesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DailyLimit != 20 {
		t.Errorf("DailyLimit = %d, want 20", cfg.DailyLimit)
	}
	if len(cfg.StudyTags) != 1 || cfg.StudyTags[0] != "#study" {
		t.Errorf("StudyTags = %v, want [#study]", cfg.StudyTags)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created on disk")
	}
}

func TestValidateRejectsEmptyNotesDirs(t *testing.T) {
	cfg := defaults()
	if err := Validate(cfg); err == nil {
		t.Error("expected error for empty NotesDirs")
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	cfg := defaults()
	cfg.NotesDirs = []string{"/tmp"}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
