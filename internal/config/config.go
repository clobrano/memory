package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

func Load(path string) (*Config, error) {
	cfg := defaults()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("create config file: %w", err)
		}
		defer f.Close()
		if err := toml.NewEncoder(f).Encode(cfg); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func Validate(c *Config) error {
	if len(c.NotesDirs) == 0 {
		return fmt.Errorf("notes_dirs must have at least one entry")
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
