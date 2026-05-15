// Package logs implements the Logs viewer (tail of mihomo log).
package logs

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
)

const ringSize = 1000

// Model is the Logs tab.
type Model struct {
	lines  []string
	paused bool
}

func New() Model { return Model{} }

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	if ev, ok := message.(msg.LogLine); ok && !m.paused {
		if len(m.lines) >= ringSize {
			m.lines = m.lines[1:]
		}
		m.lines = append(m.lines, ev.Text)
	}
	return m, nil
}

// Lines exposes the buffered lines for tests.
func (m Model) Lines() []string { return m.lines }

// TogglePause flips the pause flag.
func (m *Model) TogglePause() { m.paused = !m.paused }

// View renders the tail (most recent height-4 lines).
func (m Model) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Logs")
	pauseMark := ""
	if m.paused {
		pauseMark = " [PAUSED]"
	}
	rows := []string{header + pauseMark, ""}
	tailSize := height - 4
	if tailSize < 1 {
		tailSize = 1
	}
	start := 0
	if len(m.lines) > tailSize {
		start = len(m.lines) - tailSize
	}
	for _, l := range m.lines[start:] {
		rows = append(rows, "  "+l)
	}
	rows = append(rows, "", "[p] pause/resume")
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(strings.Join(rows, "\n"))
}
