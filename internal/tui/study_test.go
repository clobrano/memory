package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clobrano/memory/internal/config"
	"github.com/clobrano/memory/internal/db"
)

// aiModel builds a one-card session with AI enabled, on a real note file so the
// fallback paths can read it.
func aiModel(t *testing.T) Model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte("# Topic\n\nThe body of the note.\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	cfg := &config.Config{DailyLimit: 20}
	cfg.AI.Binary = "fake-ai"
	cards := []db.Card{{ID: 1, Path: path, Title: "Topic", NextDue: time.Now()}}
	m := NewModel(nil, cfg, cards, 1, 0)
	m.width, m.height = 80, 24
	return m
}

func send(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", next)
	}
	return updated, cmd
}

func keyPress(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	if s == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// A failed question generation must be shown, not swallowed: the old behaviour
// dropped to no-AI mode with nothing on screen to explain why.
func TestQuestionFailureIsReportedAndAwaitsTheUser(t *testing.T) {
	m := aiModel(t)
	m.state = stateAIQuestions
	m.aiLoading = true

	m, _ = send(t, m, aiQuestionsMsg{err: errors.New("connection refused")})

	if m.state != stateAIError {
		t.Fatalf("state = %v, want stateAIError", m.state)
	}
	if !m.aiEnabled {
		t.Error("AI was disabled before the user chose to continue without it")
	}
	view := m.View()
	for _, want := range []string{"connection refused", "fake-ai", "generate questions", "[c] Continue without AI"} {
		if !strings.Contains(view, want) {
			t.Errorf("error screen does not mention %q:\n%s", want, view)
		}
	}
}

// An AI that exits cleanly with no questions has failed too, and saying so is
// the whole point: an empty screen explains nothing.
func TestEmptyQuestionsAreReportedAsAFailure(t *testing.T) {
	m := aiModel(t)
	m.state = stateAIQuestions
	m.aiLoading = true

	m, _ = send(t, m, aiQuestionsMsg{questions: "  \n"})

	if m.state != stateAIError {
		t.Fatalf("state = %v, want stateAIError", m.state)
	}
	if !strings.Contains(m.View(), "no questions") {
		t.Errorf("error screen does not say the AI returned nothing:\n%s", m.View())
	}
}

// Continuing is the user's choice, and it lands where a no-AI session would:
// this card has no stored questions, so the note is revealed.
func TestContinueWithoutAIAfterQuestionFailure(t *testing.T) {
	m := aiModel(t)
	m.state = stateAIQuestions
	m, _ = send(t, m, aiQuestionsMsg{err: errors.New("boom")})

	m, cmd := send(t, m, keyPress("c"))

	if cmd != nil {
		t.Error("continuing without AI should not quit the session")
	}
	if m.aiEnabled {
		t.Error("aiEnabled = true after the user chose to continue without AI")
	}
	if m.state != stateReveal {
		t.Fatalf("state = %v, want stateReveal", m.state)
	}
	if m.aiErr != nil {
		t.Error("the failure is still pending after the user answered it")
	}
}

// Stopping must actually stop: the user asked not to study without AI.
func TestStopAfterQuestionFailure(t *testing.T) {
	m := aiModel(t)
	m.state = stateAIQuestions
	m, _ = send(t, m, aiQuestionsMsg{err: errors.New("boom")})

	_, cmd := send(t, m, keyPress("esc"))
	if cmd == nil {
		t.Fatal("no command returned, want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("command produced %T, want tea.QuitMsg", cmd())
	}
}

// The evaluation runs while the note is on screen. Reporting it there would
// yank the note away, so it waits for the user to ask to be graded — but it
// must never be lost, or grading would silently become manual.
func TestEvaluationFailureIsHeldUntilGrading(t *testing.T) {
	m := aiModel(t)
	m.state = stateReveal
	m.aiLoading = true

	m, _ = send(t, m, aiEvalResult{err: errors.New("model overloaded")})

	if m.state != stateReveal {
		t.Fatalf("state = %v, want the note to stay on screen", m.state)
	}
	if m.heldErr == nil {
		t.Fatal("the failure was dropped instead of held")
	}
	if !strings.Contains(m.View(), "AI evaluation failed") {
		t.Errorf("reveal screen gives no hint of the failure:\n%s", m.View())
	}

	m, _ = send(t, m, keyPress("enter"))

	if m.state != stateAIError {
		t.Fatalf("state = %v, want stateAIError once the user asked to grade", m.state)
	}
	if !strings.Contains(m.View(), "model overloaded") {
		t.Errorf("error screen does not carry the AI's own words:\n%s", m.View())
	}

	m, _ = send(t, m, keyPress("c"))
	if m.aiEnabled {
		t.Error("aiEnabled = true after continuing without AI")
	}
	if m.state != stateGrading {
		t.Errorf("state = %v, want stateGrading so the user can grade by hand", m.state)
	}
}

// A failure that arrives after the user has already moved on to grading has
// nothing to wait for, and is reported straight away.
func TestEvaluationFailureDuringGradingIsReportedAtOnce(t *testing.T) {
	m := aiModel(t)
	m.state = stateGrading
	m.aiLoading = true

	m, _ = send(t, m, aiEvalResult{err: errors.New("timeout")})

	if m.state != stateAIError {
		t.Fatalf("state = %v, want stateAIError", m.state)
	}
	if m.aiEval != nil {
		t.Error("a failed evaluation was kept as a result")
	}
}

// Once AI is off, answering stored questions must not call it again — the user
// already said they wanted to carry on without it.
func TestNoEvaluationRequestedWhenAIIsOff(t *testing.T) {
	m := aiModel(t)
	m.aiEnabled = false
	m.state = stateAIQuestions
	m.aiQuestions = "1. What is the topic?"

	m, cmd := send(t, m, keyPress("enter"))

	if cmd != nil {
		t.Error("an AI evaluation was requested with AI disabled")
	}
	if m.state != stateReveal {
		t.Errorf("state = %v, want stateReveal", m.state)
	}
	if m.aiLoading {
		t.Error("aiLoading = true with AI disabled")
	}
}
