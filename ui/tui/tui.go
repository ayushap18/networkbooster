package tui

import (
	"fmt"

	"github.com/ayush18/networkbooster/core/engine"
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI and blocks until the user quits.
// If compact is true, shows a single-line display suitable for low-power devices.
func Run(eng *engine.Engine, compact bool) error {
	model := NewModel(eng, compact)
	opts := []tea.ProgramOption{}
	if !compact {
		opts = append(opts, tea.WithAltScreen())
	}
	p := tea.NewProgram(model, opts...)
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
