package profiles

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type formField int

const (
	fieldName formField = iota
	fieldURL
)

// Form is a 2-field popup for add/edit.
type Form struct {
	name   textinput.Model
	url    textinput.Model
	active formField
}

// NewForm constructs an empty Form ready to receive keys.
func NewForm() Form {
	n := textinput.New()
	n.Placeholder = "profile name"
	n.Focus()
	u := textinput.New()
	u.Placeholder = "https://example.com/sub"
	return Form{name: n, url: u, active: fieldName}
}

// Name returns the trimmed name input.
func (f Form) Name() string { return strings.TrimSpace(f.name.Value()) }

// URL returns the trimmed URL input.
func (f Form) URL() string { return strings.TrimSpace(f.url.Value()) }

// Update routes tea.Msg to the active text input, toggling fields on Tab.
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.Type {
		case tea.KeyTab, tea.KeyShiftTab:
			if f.active == fieldName {
				f.active = fieldURL
				f.name.Blur()
				f.url.Focus()
			} else {
				f.active = fieldName
				f.url.Blur()
				f.name.Focus()
			}
			return f, nil
		}
	}
	var cmd tea.Cmd
	if f.active == fieldName {
		f.name, cmd = f.name.Update(msg)
	} else {
		f.url, cmd = f.url.Update(msg)
	}
	return f, cmd
}

// View renders the bordered popup.
func (f Form) View() string {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)
	return style.Render("Add Profile\n\nName:\n" + f.name.View() + "\n\nURL:\n" + f.url.View() + "\n\n[Tab] switch  [Enter] save  [Esc] cancel")
}
