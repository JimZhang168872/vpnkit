package settings

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/extensions"
)

// ProxyNamesFunc returns the current set of mihomo proxy names + group names
// (used for autocomplete hints in the Extensions sub-page). Caller supplies
// a closure so we don't depend directly on the api.Client.
type ProxyNamesFunc func() []string

// extensionsModel is the Settings → Extensions sub-page. Phase 4 fleshes out
// the interaction (chains/groups split, add/edit forms, apply); this file
// is the bare placeholder so settings.go compiles after the patch sub-page
// is deleted.
type extensionsModel struct {
	path      string
	ext       extensions.Extensions
	names     ProxyNamesFunc
	applyFunc func() error
	flash     string
}

func newExtensions(path string, names ProxyNamesFunc) extensionsModel {
	ext, _ := extensions.Load(path)
	return extensionsModel{path: path, ext: ext, names: names}
}

func (m extensionsModel) Update(message tea.Msg) (extensionsModel, tea.Cmd) {
	_, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	return m, nil
}

func (m extensionsModel) View(width, height int) string {
	body := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).
		Render("Extensions") +
		"\n\n  chains: " + countLine(len(m.ext.Chains)) +
		"\n  groups: " + countLine(len(m.ext.Groups)) +
		"\n\n  (Phase 4 lands the full UI)" +
		"\n\nfile: " + m.path
	if m.flash != "" {
		body += "\n\n  → " + m.flash
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func countLine(n int) string {
	if n == 0 {
		return "(none)"
	}
	return formatInt(n)
}

func formatInt(n int) string {
	return lipgloss.NewStyle().Bold(true).Render(intToStr(n))
}

func intToStr(n int) string {
	// Avoid pulling strconv in a temporary helper that goes away in Phase 4.
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
