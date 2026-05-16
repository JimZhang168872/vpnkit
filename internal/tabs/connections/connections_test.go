package connections

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/msg"
)

func TestRendersConnections(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{
		DownloadTotal: 1024,
		Items: []msg.ConnectionItem{
			{ID: "1", Host: "example.com", Port: "443", Rule: "Match", Upload: 100, Download: 200, Chains: []string{"🚀 Proxy"}},
			{ID: "2", Host: "google.com", Port: "443", Rule: "DOMAIN-SUFFIX", Upload: 50, Download: 80, Chains: []string{"DIRECT"}},
		},
	})
	view := m.View(120, 24)
	if !strings.Contains(view, "example.com") || !strings.Contains(view, "google.com") {
		t.Errorf("missing hosts:\n%s", view)
	}
}

func TestFilterHidesNonMatching(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{Items: []msg.ConnectionItem{
		{ID: "1", Host: "alpha.example.com"},
		{ID: "2", Host: "beta.example.com"},
	}})
	m.SetFilter("alpha")
	view := m.View(120, 24)
	if !strings.Contains(view, "alpha") {
		t.Errorf("alpha missing")
	}
	if strings.Contains(view, "beta") {
		t.Errorf("beta should be filtered out:\n%s", view)
	}
}

func TestFilterInputModeLiveUpdate(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{Items: []msg.ConnectionItem{
		{ID: "1", Host: "alpha.example.com"},
		{ID: "2", Host: "beta.example.com"},
	}})
	_ = m.StartFilter()
	if !m.IsFiltering() {
		t.Fatal("expected filtering=true after StartFilter")
	}
	for _, r := range "alpha" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "alpha") || strings.Contains(view, "beta") {
		t.Errorf("filter not live: %s", view)
	}
}

func TestFilterEscClears(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{Items: []msg.ConnectionItem{{ID: "1", Host: "h"}}})
	_ = m.StartFilter()
	for _, r := range "x" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsFiltering() {
		t.Error("Esc should exit filter mode")
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "h") {
		t.Errorf("Esc should clear filter and show all rows again")
	}
}

func TestFilterEnterKeepsFilter(t *testing.T) {
	m := New()
	m, _ = m.Update(msg.ConnectionsSnapshot{Items: []msg.ConnectionItem{
		{ID: "1", Host: "alpha"},
		{ID: "2", Host: "beta"},
	}})
	_ = m.StartFilter()
	for _, r := range "alp" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsFiltering() {
		t.Error("Enter should exit filter input mode")
	}
	view := m.View(120, 24)
	if !strings.Contains(view, "alpha") || strings.Contains(view, "beta") {
		t.Errorf("filter should still be applied after Enter")
	}
}
