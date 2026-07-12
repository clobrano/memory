package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/glamour"
)

type sessionState int

const (
	statePreSession    sessionState = iota
	stateRecall
	stateAIQuestions // AI mode only: show questions, collect answers
	stateReveal
	stateGrading
	stateSessionSummary
	stateDone
)

type keyMap struct {
	Enter  key.Binding
	Quit   key.Binding
	One    key.Binding
	Two    key.Binding
	Three  key.Binding
	Cap    key.Binding
	All    key.Binding
	Accept key.Binding
	Override key.Binding
	Up     key.Binding
	Down   key.Binding
	Continue key.Binding
}

var keys = keyMap{
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Quit:     key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
	One:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "all correct")),
	Two:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "partially correct")),
	Three:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "needs review")),
	Cap:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cap session")),
	All:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "review all")),
	Accept:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "accept")),
	Override: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "override")),
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Continue: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "continue")),
}

func renderMarkdown(content string) string {
	out, err := glamour.Render(content, "dark")
	if err != nil {
		return content
	}
	return out
}
