package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	gapLen := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gapLen < 0 {
		gapLen = 0
	}
	gap := strings.Repeat(" ", gapLen)
	return lipgloss.NewStyle().Reverse(true).Width(width).Render(left + gap + right)
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
