// Package stub provides a placeholder tab model used until a real implementation lands.
package stub

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is a stateless placeholder tab.
type Model struct {
	Name string
}

// New returns a stub for a named tab.
func New(name string) Model { return Model{Name: name} }

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update is a no-op.
func (m Model) Update(tea.Msg) (Model, tea.Cmd) { return m, nil }

// View renders a centered placeholder.
func (m Model) View(width, height int) string {
	body := fmt.Sprintf("%s — coming in a later phase", m.Name)
	style := lipgloss.NewStyle().Width(width).Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("240"))
	return style.Render(body)
}
