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
// Returns: questions, suggestions, isCached, noteChanged, error
func AskQuestions(cfg config.AIConfig, noteContent string, dbConn *sql.DB, cardID int64) (questions, suggestions string, isCached, noteChanged bool, err error) {
	currentHash := cache.ComputeNoteHash(noteContent)

	// Reuse cached questions only while the note content is unchanged; a changed
	// note must be re-asked, otherwise the questions would describe stale content.
	if dbConn != nil && cardID > 0 {
		card, cacheErr := db.GetCardByID(dbConn, cardID)
		if cacheErr == nil && card != nil && card.CachedQuestions != "" && card.NoteContentHash == currentHash {
			questions, suggestions = splitQuestions(card.CachedQuestions)
			return questions, suggestions, true, false, nil
		}
	}

	// Cache miss: generate new questions
	template := loadPrompt(cfg.QuestionPromptFile, defaultQuestionsPrompt)
	prompt := strings.ReplaceAll(template, "{{NOTE_CONTENT}}", noteContent)
	output, err := invoke(cfg, prompt)
	if err != nil {
		return "", "", false, false, err
	}

	questions, suggestions = splitQuestions(output)

	// Store in cache if database is provided
	if dbConn != nil && cardID > 0 {
		_ = db.UpdateCachedQuestions(dbConn, cardID, currentHash, output)
	}

	return questions, suggestions, false, false, nil
}

// SetQuestions stores questions written by hand for a card. They are stamped
// with the note's current hash, so the next review treats them as up to date
// and serves them without calling the AI. Editing the note invalidates them the
// same way it invalidates generated ones: with AI enabled the next review
// regenerates and overwrites, without it they are shown with a stale warning.
func SetQuestions(dbConn *sql.DB, cardID int64, noteContent, questions string) error {
	if dbConn == nil || cardID <= 0 {
		return fmt.Errorf("no card to store questions for")
	}
	questions = strings.TrimSpace(questions)
	if questions == "" {
		return fmt.Errorf("questions are empty")
	}
	return db.UpdateCachedQuestions(dbConn, cardID, cache.ComputeNoteHash(noteContent), questions)
}

// GetCachedQuestions retrieves cached questions if available
// Returns: questions, suggestions, isCached, noteChanged, error
func GetCachedQuestions(dbConn *sql.DB, cardID int64, noteContent string) (questions, suggestions string, isCached, noteChanged bool, err error) {
	if dbConn == nil || cardID <= 0 {
		return "", "", false, false, nil
	}

	currentHash := cache.ComputeNoteHash(noteContent)
	card, err := db.GetCardByID(dbConn, cardID)
	if err != nil || card == nil || card.CachedQuestions == "" {
		return "", "", false, false, err
	}

	questions, suggestions = splitQuestions(card.CachedQuestions)
	return questions, suggestions, true, card.NoteContentHash != currentHash, nil
}

// splitQuestions separates the questions block from the trailing note suggestion.
func splitQuestions(raw string) (questions, suggestions string) {
	parts := strings.SplitN(raw, "\n---\n", 2)
	questions = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		suggestions = strings.TrimSpace(parts[1])
	}
	return questions, suggestions
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
