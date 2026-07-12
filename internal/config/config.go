package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type AIConfig struct {
	Binary             string   `toml:"binary"`
	Args               []string `toml:"args"`
	QuestionPromptFile string   `toml:"question_prompt_file"`
	EvaluatePromptFile string   `toml:"evaluate_prompt_file"`
}

type Config struct {
	NotesDirs  []string `toml:"notes_dirs"`
	StudyTags  []string `toml:"study_tags"`
	DailyLimit int      `toml:"daily_limit"`
	AI         AIConfig `toml:"ai"`
}

func defaults() *Config {
	return &Config{
		StudyTags:  []string{"#study"},
		DailyLimit: 20,
	}
}

const defaultConfigTemplate = `# memory — spaced-repetition CLI config
# Full reference: https://github.com/clobrano/memory

# Directories to scan for markdown notes.
# Add one or more absolute paths to your vault(s). Required.
# notes_dirs = ["~/notes", "~/obsidian-vault"]
notes_dirs = []

# Tags that mark a note as a study card (case-sensitive). Default: ["#study"]
study_tags = ["#study"]

# Maximum cards to review in a single session. Default: 20
daily_limit = 20

[ai]
# Path or name of an AI CLI binary to enable AI question generation and
# evaluation. Leave commented out (or empty) to disable AI mode entirely.
# Examples: "claude", "ollama", "/usr/local/bin/my-ai-tool"
# binary = "claude"

# Arguments passed to the binary before the prompt is piped to stdin.
# args = ["--model", "claude-opus-4-5", "-p"]

# Override the default prompt templates (written to the prompts/ dir below).
# question_prompt_file = ""
# evaluate_prompt_file = ""
`

func Load(path string) (*Config, error) {
	cfg := defaults()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}
		if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o644); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Config created at %s — edit notes_dirs before running.\n", path)
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	home, _ := os.UserHomeDir()
	for i, dir := range cfg.NotesDirs {
		cfg.NotesDirs[i] = expandTilde(dir, home)
	}
	return cfg, nil
}

func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func Validate(c *Config) error {
	if len(c.NotesDirs) == 0 {
		return fmt.Errorf("notes_dirs is empty — open your config file and add at least one vault directory")
	}
	if c.DailyLimit <= 0 {
		return fmt.Errorf("daily_limit must be > 0")
	}
	if c.AI.Binary != "" {
		if _, err := exec.LookPath(c.AI.Binary); err != nil {
			return fmt.Errorf("ai.binary %q not found or not executable: %w", c.AI.Binary, err)
		}
	}
	return nil
}
