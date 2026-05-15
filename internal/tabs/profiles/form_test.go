package profiles

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFormCollectsFields(t *testing.T) {
	f := NewForm()
	for _, r := range "main" {
		f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "https://x" {
		f, _ = f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if f.Name() != "main" {
		t.Errorf("name: %q", f.Name())
	}
	if f.URL() != "https://x" {
		t.Errorf("url: %q", f.URL())
	}
}
