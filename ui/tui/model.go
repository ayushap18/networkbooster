package tui

import (
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

// Model is the bubbletea model for the NetworkBooster TUI.
type Model struct {
	engine   *engine.Engine
	compact  bool
	quitting bool
	width    int
	height   int
}

// NewModel creates a new TUI model.
func NewModel(eng *engine.Engine, compact bool) Model {
	return Model{
		engine:  eng,
		compact: compact,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tickCmd()
	}
	return m, nil
}
