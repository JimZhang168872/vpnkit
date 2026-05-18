package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/tabs/viewport"
)

func (m Model) renderStatusBar(width int) string {
	left := fmt.Sprintf(" %s  ↑ %s/s  ↓ %s/s ",
		runDot(m.dashboard),
		fmtRate(m.dashboard.UpHistoryLast()),
		fmtRate(m.dashboard.DownHistoryLast()),
	)
	right := " ?:help q:quit "
	if m.flash != "" {
		right = " " + m.flash + " "
	}
	badge := ""
	if m.updateBadge != "" {
		badge = lipgloss.NewStyle().Faint(true).Render(" " + m.updateBadge + " ")
	}
	// Truncate the flash/help text to whatever space remains after the
	// left dashboard segment + the update badge + a 1-char minimum gap.
	// Pre-rc.7 a long flash made lipgloss wrap to a SECOND line, which
	// overflowed the intended single-line status bar and eat into the
	// content row above — QA round-2 caught this as "sidebar entries
	// disappear after long flash". Truncating keeps the bar single-line.
	rightBudget := width - lipgloss.Width(left) - lipgloss.Width(badge) - 1
	if rightBudget < 8 {
		rightBudget = 8
	}
	if lipgloss.Width(right) > rightBudget {
		right = viewport.TruncateDisplay(right, rightBudget)
	}
	gapLen := width - lipgloss.Width(left) - lipgloss.Width(badge) - lipgloss.Width(right)
	if gapLen < 0 {
		gapLen = 0
	}
	gap := strings.Repeat(" ", gapLen)
	// MaxHeight(1) defends against ANY remaining wrap path (e.g. embedded
	// newline in a flash message we forgot to sanitize). Without it,
	// terminal wraps push the rest of the TUI up by one row.
	return lipgloss.NewStyle().Reverse(true).Width(width).MaxHeight(1).Render(left + badge + gap + right)
}

func runDot(d interface{ UpHistoryLast() int64 }) string {
	if d.UpHistoryLast() > 0 {
		return "🟢"
	}
	return "⚪"
}

func fmtRate(n int64) string {
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
