package ai

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/clobrano/memory/internal/config"
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

func AskQuestions(cfg config.AIConfig, noteContent string) (questions, suggestions string, err error) {
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
	return questions, suggestions, nil
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
