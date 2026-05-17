// Package logs implements the Logs viewer (tail of mihomo log).
package logs

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/tabs/viewport"
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

// View renders the tail; defaults to focused for direct callers (tests).
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused = View + focus dot prefix.
func (m Model) ViewFocused(width, height int, focused bool) string {
	header := viewport.FocusDot(focused) +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Logs")
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
	innerWidth := width - 6 // -4 padding -2 prefix slack
	if innerWidth < 10 {
		innerWidth = 10
	}
	for _, l := range m.lines[start:] {
		rows = append(rows, "  "+viewport.TruncateDisplay(l, innerWidth))
	}
	rows = append(rows, "", "[p] pause/resume")
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		Padding(1, 2).Render(strings.Join(rows, "\n"))
}
