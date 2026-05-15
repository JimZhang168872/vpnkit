package settings

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/paths"
)

type patchModel struct {
	path  string
	area  textarea.Model
	flash string
}

// newPatch constructs a patchModel, loading existing patch.yaml if present.
func newPatch(p paths.XDG) patchModel {
	ta := textarea.New()
	ta.Placeholder = "# YAML overlay (deep-merged onto subscription + rules)"
	ta.ShowLineNumbers = true
	ta.SetWidth(80)
	ta.SetHeight(18)
	pPath := filepath.Join(p.MihomoConfig, "patch.yaml")
	m := patchModel{path: pPath, area: ta}
	if data, err := os.ReadFile(pPath); err == nil {
		m.area.SetValue(string(data))
	}
	return m
}

// Content returns the current textarea value.
func (m patchModel) Content() string {
	return m.area.Value()
}

// SetContent updates the textarea with new YAML content.
func (m *patchModel) SetContent(s string) {
	m.area.SetValue(s)
}

// Save writes the textarea content to patch.yaml.
func (m patchModel) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.path, []byte(m.area.Value()), 0o600)
}

// Update handles keyboard input and textarea updates.
func (m patchModel) Update(message tea.Msg) (patchModel, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok {
		if km.Type == tea.KeyCtrlS {
			if err := m.Save(); err != nil {
				m.flash = "save: " + err.Error()
			} else {
				m.flash = "saved"
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.area, cmd = m.area.Update(message)
	return m, cmd
}

// View renders the patch editor with header, textarea, and status.
func (m patchModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).
		Render("Patch Editor (~/.config/mihomo/patch.yaml)")
	body := header + "\n" + m.area.View() + "\n\n  [Ctrl+S] save"
	if m.flash != "" {
		body += "  → " + m.flash
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}
