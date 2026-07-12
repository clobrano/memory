package ai

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/clobrano/memory/internal/config"
)

//go:embed prompts/questions.txt
var defaultQuestionsPrompt string

//go:embed prompts/evaluate.txt
var defaultEvaluatePrompt string

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

func AskQuestions(cfg config.AIConfig, noteContent string) (string, error) {
	template := loadPrompt(cfg.QuestionPromptFile, defaultQuestionsPrompt)
	prompt := strings.ReplaceAll(template, "{{NOTE_CONTENT}}", noteContent)
	return invoke(cfg, prompt)
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
