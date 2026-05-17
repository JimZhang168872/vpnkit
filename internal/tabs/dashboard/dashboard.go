// Package dashboard implements the first vpnkit tab: live traffic + service status.
package dashboard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/msg"
	"vpnkit/internal/tabs/viewport"
)

const historySize = 60

// Model holds the dashboard's local state.
type Model struct {
	upHist, downHist []int64
	lastUp, lastDown int64
	mihomoVer        string
	mode             string
	running          bool
}

// New returns an empty dashboard model.
func New() Model {
	return Model{
		upHist:   make([]int64, 0, historySize),
		downHist: make([]int64, 0, historySize),
		mode:     "rule",
	}
}

// Init satisfies tea.Model.
func (Model) Init() tea.Cmd { return nil }

// Update absorbs messages.
func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	switch v := message.(type) {
	case msg.Traffic:
		m.lastUp = v.Up
		m.lastDown = v.Down
		m.upHist = pushRing(m.upHist, v.Up, historySize)
		m.downHist = pushRing(m.downHist, v.Down, historySize)
	case msg.Version:
		if v.Err == nil {
			m.mihomoVer = v.Version
		}
	case msg.ServiceStatus:
		m.running = v.Running
	}
	return m, nil
}

// UpHistory exposes the up-traffic ring (for tests).
func (m Model) UpHistory() []int64 { return m.upHist }

// DownHistory exposes the down-traffic ring (used by status bar).
func (m Model) DownHistory() []int64 { return m.downHist }

// UpHistoryLast returns the most recent up rate or 0.
func (m Model) UpHistoryLast() int64 {
	if len(m.upHist) == 0 {
		return 0
	}
	return m.upHist[len(m.upHist)-1]
}

// DownHistoryLast returns the most recent down rate or 0.
func (m Model) DownHistoryLast() int64 {
	if len(m.downHist) == 0 {
		return 0
	}
	return m.downHist[len(m.downHist)-1]
}

// View renders the dashboard within (width, height). Defaults to focused
// presentation for direct callers (tests). The app's main view passes the
// app-level focus state via ViewFocused.
func (m Model) View(width, height int) string {
	return m.ViewFocused(width, height, true)
}

// ViewFocused is View + focus state for the focus-dot prefix on the header.
func (m Model) ViewFocused(width, height int, focused bool) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	body := fmt.Sprintf(
		"%s%s\n\n  Status : %s\n  Version: %s\n  Mode   : %s\n\n  ↑ %s/s\n  ↓ %s/s\n",
		viewport.FocusDot(focused),
		headerStyle.Render("Mihomo"),
		runStr(m.running),
		fallback(m.mihomoVer, "unknown"),
		m.mode,
		humanRate(m.lastUp),
		humanRate(m.lastDown),
	)
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func pushRing(buf []int64, v int64, max int) []int64 {
	if len(buf) >= max {
		buf = buf[1:]
	}
	return append(buf, v)
}

func runStr(b bool) string {
	if b {
		return "● running"
	}
	return "○ stopped"
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func humanRate(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
	)
	switch {
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
