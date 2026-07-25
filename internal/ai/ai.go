package ai

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/clobrano/memory/internal/cache"
	"github.com/clobrano/memory/internal/config"
	"github.com/clobrano/memory/internal/db"
)

//go:embed prompts/questions.txt
var defaultQuestionsPrompt string

//go:embed prompts/evaluate.txt
var defaultEvaluatePrompt string

// EnsureDefaultPrompts writes the embedded prompt templates to <dir>/prompts/
// if they don't already exist, and returns their paths.
func EnsureDefaultPrompts(dir string) (questionsPath, evaluatePath string, err error) {
	promptsDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create prompts dir: %w", err)
	}
	questionsPath = filepath.Join(promptsDir, "questions.txt")
	evaluatePath = filepath.Join(promptsDir, "evaluate.txt")
	for _, f := range []struct {
		path    string
		content string
	}{
		{questionsPath, defaultQuestionsPrompt},
		{evaluatePath, defaultEvaluatePrompt},
	} {
		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
				return "", "", fmt.Errorf("write %s: %w", f.path, err)
			}
		}
	}
	return questionsPath, evaluatePath, nil
}

// ResetPrompts overwrites the prompt files in <dir>/prompts/ with the
// embedded defaults, regardless of whether they already exist.
func ResetPrompts(dir string) error {
	promptsDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return fmt.Errorf("create prompts dir: %w", err)
	}
	files := []struct {
		name    string
		content string
	}{
		{"questions.txt", defaultQuestionsPrompt},
		{"evaluate.txt", defaultEvaluatePrompt},
	}
	for _, f := range files {
		path := filepath.Join(promptsDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
		fmt.Printf("reset %s\n", path)
	}
	return nil
}

func loadPrompt(path, fallback string) string {
	if path == "" {
		return fallback
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(b)
}

func invoke(cfg config.AIConfig, prompt string) (string, error) {
	cmd := exec.Command(cfg.Binary, cfg.Args...)
	cmd.Stdin = strings.NewReader(prompt)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, errBuf.String())
	}
	return out.String(), nil
}

// AskQuestions generates or retrieves cached questions for a note
// If database and cardID are provided, it attempts to reuse cached questions if the note hasn't changed
func AskQuestions(cfg config.AIConfig, noteContent string, dbConn *sql.DB, cardID int64) (questions, suggestions string, err error) {
	// Compute hash of note content
	currentHash := cache.ComputeNoteHash(noteContent)

	// Check cache if database is provided
	if dbConn != nil && cardID > 0 {
		card, err := db.GetCardByID(dbConn, cardID)
		if err == nil && card != nil && card.NoteContentHash == currentHash && card.CachedQuestions != "" {
			// Cache hit: return cached questions
			parts := strings.SplitN(card.CachedQuestions, "\n---\n", 2)
			questions = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				suggestions = strings.TrimSpace(parts[1])
			}
			return questions, suggestions, nil
		}
	}

	// Cache miss: generate new questions
	template := loadPrompt(cfg.QuestionPromptFile, defaultQuestionsPrompt)
	prompt := strings.ReplaceAll(template, "{{NOTE_CONTENT}}", noteContent)
	output, err := invoke(cfg, prompt)
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(output, "\n---\n", 2)
	questions = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		suggestions = strings.TrimSpace(parts[1])
	}

	// Store in cache if database is provided
	if dbConn != nil && cardID > 0 {
		_ = db.UpdateCachedQuestions(dbConn, cardID, currentHash, output)
	}

	return questions, suggestions, nil
}

// AskQuestionsNoCache generates questions without caching (for backward compatibility)
func AskQuestionsNoCache(cfg config.AIConfig, noteContent string) (questions, suggestions string, err error) {
	return AskQuestions(cfg, noteContent, nil, 0)
}

func Evaluate(cfg config.AIConfig, noteContent, qaTranscript string) (grade, rationale string, err error) {
	template := loadPrompt(cfg.EvaluatePromptFile, defaultEvaluatePrompt)
	prompt := strings.ReplaceAll(template, "{{NOTE_CONTENT}}", noteContent)
	prompt = strings.ReplaceAll(prompt, "{{QA_TRANSCRIPT}}", qaTranscript)

	output, err := invoke(cfg, prompt)
	if err != nil {
		return "", "", err
	}

	lines := strings.SplitN(strings.TrimSpace(output), "\n", 2)
	grade = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		rationale = strings.TrimSpace(lines[1])
	}
	return grade, rationale, nil
}
