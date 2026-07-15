package tui

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/clobrano/memory/internal/ai"
	"github.com/clobrano/memory/internal/config"
	"github.com/clobrano/memory/internal/db"
	"github.com/clobrano/memory/internal/fsrs"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Border(lipgloss.RoundedBorder()).Padding(0, 1)
	hintStyle    = lipgloss.NewStyle().Faint(true)
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	boldStyle    = lipgloss.NewStyle().Bold(true)
	aiPanelStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("12"))
)

type gradeResult struct {
	card  db.Card
	grade fsrs.Grade
}

type aiQuestionsMsg struct {
	questions   string
	suggestions string
	err         error
}

type aiEvalResult struct {
	grade     string
	rationale string
	err       error
}

type Model struct {
	db         *sql.DB
	cfg        *config.Config
	cards      []db.Card
	index      int
	state      sessionState
	dailyLimit int
	aiEnabled  bool

	viewport viewport.Model // note content (reveal) and AI questions
	evalVP   viewport.Model // AI evaluation result (grading)
	textarea textarea.Model

	// pre-session stats
	vaultTotal int
	streak     int

	// grading
	reviewed      []gradeResult
	aiQuestions   string
	aiSuggestions string
	aiEval        *aiEvalResult
	aiLoading     bool

	// quit confirmation
	confirmQuit bool

	err error
	width  int
	height int
}

func NewModel(database *sql.DB, cfg *config.Config, cards []db.Card, vaultTotal, streak int) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your answer here (Alt-Enter for new line, Enter to submit)..."
	ta.CharLimit = 4000
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"), key.WithHelp("alt+enter", "new line"))

	vp := viewport.New(80, 20)
	evp := viewport.New(80, 20)

	aiEnabled := cfg.AI.Binary != ""

	return Model{
		db:         database,
		cfg:        cfg,
		cards:      cards,
		state:      statePreSession,
		dailyLimit: cfg.DailyLimit,
		aiEnabled:  aiEnabled,
		viewport:   vp,
		evalVP:     evp,
		textarea:   ta,
		vaultTotal: vaultTotal,
		streak:     streak,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) currentCard() *db.Card {
	if m.index < len(m.cards) {
		return &m.cards[m.index]
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.evalVP.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = m.viewportHeight()
		m.evalVP.Height = m.viewportHeightForGrading()
		if m.state == stateAIQuestions {
			m.viewport.Height = m.aiQuestionsViewportHeight()
			m.textarea.SetHeight(m.aiAnswerHeight())
		}

	case aiQuestionsMsg:
		m.aiLoading = false
		if msg.err != nil || msg.questions == "" {
			// AI failed — skip to reveal without questions
			m.aiEnabled = false
			card := m.currentCard()
			if card != nil {
				content, _ := readNoteContent(card.Path)
				m.viewport.Height = m.viewportHeightForReveal()
				m.viewport.SetContent(renderMarkdown(content))
			}
			m.state = stateReveal
		} else {
			w := m.width - 2
			if w < 20 {
				w = 20
			}
			m.aiQuestions = msg.questions
			m.aiSuggestions = msg.suggestions
			vpContent := wordwrap.String(msg.questions, w)
			if msg.suggestions != "" {
				vpContent += "\n\n" + hintStyle.Render("--- note suggestion ---\n"+wordwrap.String(msg.suggestions, w))
			}
			m.viewport.Height = m.aiQuestionsViewportHeight()
			m.viewport.SetContent(vpContent)
			m.viewport.GotoTop()
			m.textarea.SetHeight(m.aiAnswerHeight())
			m.textarea.Focus()
		}
		return m, nil

	case aiEvalResult:
		m.aiLoading = false
		m.aiEval = &msg
		if msg.err != nil {
			m.aiEnabled = false
		} else {
			w := m.width - 2
			if w < 20 {
				w = 20
			}
			content := boldStyle.Render("Suggested: "+msg.grade) + "\n\n" +
				wordwrap.String(msg.rationale, w)
			m.evalVP.Height = m.viewportHeightForGrading()
			m.evalVP.SetContent(content)
			m.evalVP.GotoTop()
		}
		return m, nil

	case tea.KeyMsg:
		if m.confirmQuit {
			switch msg.String() {
			case "y", "Y":
				return m, tea.Quit
			case "s", "S":
				m.confirmQuit = false
				return m.skipCard()
			default:
				m.confirmQuit = false
				return m, nil
			}
		}

		switch m.state {
		case statePreSession:
			return m.updatePreSession(msg)
		case stateRecall:
			return m.updateRecall(msg)
		case stateAIQuestions:
			return m.updateAIQuestions(msg)
		case stateReveal:
			return m.updateReveal(msg)
		case stateGrading:
			return m.updateGrading(msg)
		case stateSessionSummary:
			return m.updateSummary(msg)
		}
	}

	if m.state == stateAIQuestions {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) updatePreSession(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	due := len(m.cards)
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Cap) && due > m.dailyLimit:
		m.cards = m.cards[:m.dailyLimit]
		return m.startSession()
	case key.Matches(msg, keys.All) && due > m.dailyLimit:
		return m.startSession()
	case key.Matches(msg, keys.Enter):
		if due > m.dailyLimit {
			m.cards = m.cards[:m.dailyLimit]
		}
		return m.startSession()
	}
	return m, nil
}

func (m Model) startSession() (tea.Model, tea.Cmd) {
	if len(m.cards) == 0 {
		m.state = stateSessionSummary
		return m, nil
	}
	if m.aiEnabled {
		return m.beginCardWithAI()
	}
	m.state = stateRecall
	return m, nil
}

// beginCardWithAI immediately starts fetching questions for the current card.
func (m Model) beginCardWithAI() (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil {
		m.state = stateSessionSummary
		return m, nil
	}
	m.aiQuestions = ""
	m.aiSuggestions = ""
	m.aiEval = nil
	m.aiLoading = true
	m.textarea.Reset()
	m.state = stateAIQuestions
	content, _ := readNoteContent(card.Path)
	return m, fetchAIQuestions(m.cfg.AI, content)
}

func (m Model) updateRecall(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.confirmQuit = true
		return m, nil
	case key.Matches(msg, keys.Enter):
		card := m.currentCard()
		if card == nil {
			m.state = stateSessionSummary
			return m, nil
		}
		content, _ := readNoteContent(card.Path)
		m.viewport.Height = m.viewportHeightForReveal()
		m.viewport.SetContent(renderMarkdown(content))
		m.viewport.GotoTop()
		m.state = stateReveal
	}
	return m, nil
}

func fetchAIQuestions(cfg config.AIConfig, content string) tea.Cmd {
	return func() tea.Msg {
		q, s, err := ai.AskQuestions(cfg, content)
		return aiQuestionsMsg{questions: q, suggestions: s, err: err}
	}
}

func (m Model) updateAIQuestions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.confirmQuit = true
		return m, nil
	case key.Matches(msg, keys.Enter):
		if m.aiLoading {
			return m, nil // still waiting for questions
		}
		answer := m.textarea.Value()
		card := m.currentCard()
		content, _ := readNoteContent(card.Path)
		m.viewport.Height = m.viewportHeightForReveal()
		m.viewport.SetContent(renderMarkdown(content))
		m.viewport.GotoTop()
		transcript := m.aiQuestions + "\n\nAnswer:\n" + answer
		m.aiLoading = true
		m.state = stateReveal
		return m, fetchAIEval(m.cfg.AI, content, transcript)
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) updateReveal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.confirmQuit = true
		return m, nil
	case key.Matches(msg, keys.Enter):
		m.state = stateGrading
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func fetchAIEval(cfg config.AIConfig, content, transcript string) tea.Cmd {
	return func() tea.Msg {
		grade, rationale, err := ai.Evaluate(cfg, content, transcript)
		return aiEvalResult{grade: grade, rationale: rationale, err: err}
	}
}

func (m Model) updateGrading(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// AI eval panel: accept or override
	if m.aiEval != nil {
		switch msg.String() {
		case "esc":
			m.confirmQuit = true
			return m, nil
		case "a", "A":
			grade := aiGradeToFSRS(m.aiEval.grade)
			return m.applyGrade(grade)
		case "o", "O":
			m.aiEval = nil // show manual grading
			return m, nil
		}
		var cmd tea.Cmd
		m.evalVP, cmd = m.evalVP.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, keys.Quit):
		m.confirmQuit = true
		return m, nil
	case key.Matches(msg, keys.One):
		return m.applyGrade(fsrs.GradeAllCorrect)
	case key.Matches(msg, keys.Two):
		return m.applyGrade(fsrs.GradePartiallyCorrect)
	case key.Matches(msg, keys.Three):
		return m.applyGrade(fsrs.GradeNeedsReview)
	}
	return m, nil
}

func (m Model) applyGrade(grade fsrs.Grade) (tea.Model, tea.Cmd) {
	card := m.currentCard()
	if card == nil {
		m.state = stateSessionSummary
		return m, nil
	}
	updated, err := fsrs.Schedule(*card, grade, time.Now())
	if err == nil {
		_ = db.UpdateCardSchedule(m.db, updated)
		_ = db.InsertReview(m.db, db.Review{
			CardID:     updated.ID,
			ReviewedAt: time.Now(),
			Grade:      grade.String(),
			Rating:     grade.Rating(),
		})
	}
	m.reviewed = append(m.reviewed, gradeResult{card: updated, grade: grade})
	m.index++
	if m.index >= len(m.cards) {
		m.state = stateSessionSummary
		return m, nil
	}
	if m.aiEnabled {
		return m.beginCardWithAI()
	}
	m.state = stateRecall
	m.aiQuestions = ""
	m.aiSuggestions = ""
	m.aiEval = nil
	return m, nil
}

func (m Model) skipCard() (tea.Model, tea.Cmd) {
	m.index++
	if m.index >= len(m.cards) {
		m.state = stateSessionSummary
		return m, nil
	}
	if m.aiEnabled {
		return m.beginCardWithAI()
	}
	m.state = stateRecall
	m.aiQuestions = ""
	m.aiSuggestions = ""
	m.aiEval = nil
	return m, nil
}

func (m Model) updateSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit), key.Matches(msg, keys.Enter):
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() string {
	if m.confirmQuit {
		return "\nQuit session? [y] Quit  [s] Skip card  [any] Cancel: "
	}
	switch m.state {
	case statePreSession:
		return m.viewPreSession()
	case stateRecall:
		return m.viewRecall()
	case stateAIQuestions:
		return m.viewAIQuestions()
	case stateReveal:
		return m.viewReveal()
	case stateGrading:
		return m.viewGrading()
	case stateSessionSummary:
		return m.viewSummary()
	}
	return ""
}

func (m Model) viewPreSession() string {
	due := len(m.cards)
	var b strings.Builder
	b.WriteString(boldStyle.Render("Memory — Study Session") + "\n\n")
	fmt.Fprintf(&b, "  Due today:    %d\n", due)
	fmt.Fprintf(&b, "  Vault total:  %d\n", m.vaultTotal)
	fmt.Fprintf(&b, "  Streak:       %d days\n", m.streak)
	fmt.Fprintf(&b, "  Daily limit:  %d\n\n", m.dailyLimit)
	if due == 0 {
		b.WriteString("  Nothing due today! Great work.\n\n")
		b.WriteString(hintStyle.Render("[Esc] Quit"))
		return b.String()
	}
	if due > m.dailyLimit {
		b.WriteString(warningStyle.Render(fmt.Sprintf("  Warning: %d cards due (limit: %d)\n", due, m.dailyLimit)))
		b.WriteString(hintStyle.Render("  [Enter] Cap session  [a] Review all  [Esc] Quit\n"))
	} else {
		b.WriteString(hintStyle.Render("  [Enter] Start  [Esc] Quit\n"))
	}
	if m.aiEnabled {
		b.WriteString("\n  " + boldStyle.Render("[AI mode: on]"))
	}
	return b.String()
}

func (m Model) viewRecall() string {
	card := m.currentCard()
	if card == nil {
		return "No card."
	}
	progress := fmt.Sprintf("[%d/%d]", m.index+1, len(m.cards))
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", hintStyle.Render(progress))
	b.WriteString(titleStyle.Render(card.Title) + "\n\n")
	b.WriteString(hintStyle.Render("Try to recall the content, then press ENTER  [Esc] Skip/Quit"))
	return b.String()
}

func (m Model) viewAIQuestions() string {
	card := m.currentCard()
	if card == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", hintStyle.Render(fmt.Sprintf("[%d/%d] %s", m.index+1, len(m.cards), card.Title)))
	if m.aiLoading {
		b.WriteString(hintStyle.Render("  AI is generating questions..."))
		return b.String()
	}
	b.WriteString(m.viewport.View() + "\n")
	b.WriteString(m.textarea.View() + "\n")
	b.WriteString(hintStyle.Render("[Enter] Submit answers and reveal note  [↑/↓] Scroll  [Esc] Skip/Quit"))
	return b.String()
}

func (m Model) viewReveal() string {
	card := m.currentCard()
	if card == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", hintStyle.Render(fmt.Sprintf("[%d/%d] %s", m.index+1, len(m.cards), card.Title)))
	b.WriteString(m.viewport.View() + "\n")
	hint := "[Enter] Grade  [↑/↓] Scroll  [Esc] Skip/Quit"
	if m.aiEnabled && m.aiLoading {
		hint = "[Enter] Grade  [↑/↓] Scroll  [Esc] Skip/Quit  · AI evaluating in background…"
	}
	b.WriteString(hintStyle.Render(hint))
	return b.String()
}

func (m Model) viewGrading() string {
	card := m.currentCard()
	if card == nil {
		return ""
	}

	if m.aiEnabled {
		if m.aiLoading {
			return boldStyle.Render("AI Evaluation") + "\n\n" +
				hintStyle.Render("  AI is evaluating your answers...")
		}
		if m.aiEval != nil && m.aiEval.err == nil {
			var b strings.Builder
			b.WriteString(boldStyle.Render("AI Evaluation") + "\n")
			b.WriteString(m.evalVP.View() + "\n")
			b.WriteString(hintStyle.Render("[a] Accept  [o] Override  [↑/↓] Scroll  [Esc] Skip/Quit"))
			return b.String()
		}
		// AI failed — fall through to manual grading
	}

	var b strings.Builder
	b.WriteString(boldStyle.Render("How did you do?") + "\n\n")
	b.WriteString("  [1] All correct\n")
	b.WriteString("  [2] Partially correct\n")
	b.WriteString("  [3] Needs review\n\n")
	b.WriteString(hintStyle.Render("[Esc] Skip/Quit"))
	return b.String()
}

func (m Model) viewSummary() string {
	var b strings.Builder
	b.WriteString(boldStyle.Render("Session Complete!") + "\n\n")
	fmt.Fprintf(&b, "  Cards reviewed: %d\n\n", len(m.reviewed))

	counts := map[string]int{}
	var earliest time.Time
	for _, r := range m.reviewed {
		counts[r.grade.String()]++
		if earliest.IsZero() || (!r.card.NextDue.IsZero() && r.card.NextDue.Before(earliest)) {
			earliest = r.card.NextDue
		}
	}

	if c := counts["All correct"]; c > 0 {
		fmt.Fprintf(&b, "  All correct:       %d\n", c)
	}
	if c := counts["Partially correct"]; c > 0 {
		fmt.Fprintf(&b, "  Partially correct: %d\n", c)
	}
	if c := counts["Needs review"]; c > 0 {
		fmt.Fprintf(&b, "  Needs review:      %d\n", c)
	}
	if !earliest.IsZero() {
		fmt.Fprintf(&b, "\n  Next review: %s\n", earliest.Format("2006-01-02"))
	}
	b.WriteString("\n" + hintStyle.Render("[Enter/q] Exit"))
	return b.String()
}

// viewportHeight helpers — each reserves lines for the chrome around the viewport.
func (m Model) viewportHeight() int { return m.viewportHeightForReveal() }

func (m Model) viewportHeightForReveal() int {
	h := m.height - 4 // header(1) + hint(1) + padding(2)
	if h < 5 {
		h = 5
	}
	return h
}

// aiContentHeight is the total lines available to split between questions and answer.
func (m Model) aiContentHeight() int {
	chrome := 3 // header(1) + hint(1) + padding(1)
	h := m.height - chrome
	if h < 6 {
		h = 6
	}
	return h
}

func (m Model) aiQuestionsViewportHeight() int {
	return m.aiContentHeight() / 2
}

func (m Model) aiAnswerHeight() int {
	return m.aiContentHeight() - m.aiQuestionsViewportHeight()
}

func (m Model) viewportHeightForGrading() int {
	h := m.height - 4 // header(1) + hint(1) + padding(2)
	if h < 3 {
		h = 3
	}
	return h
}

func readNoteContent(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func aiGradeToFSRS(grade string) fsrs.Grade {
	switch strings.ToLower(grade) {
	case "all correct":
		return fsrs.GradeAllCorrect
	case "partially correct":
		return fsrs.GradePartiallyCorrect
	default:
		return fsrs.GradeNeedsReview
	}
}
