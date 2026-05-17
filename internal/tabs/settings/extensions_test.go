package settings

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/extensions"
)

func TestExtensionsViewListsChainsAndGroups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
		Groups: []extensions.Group{
			{Name: "G1", Type: "select", Proxies: []string{"DIRECT"}},
		},
	})
	m := newExtensions(path, func() []string { return []string{"A", "B"} })
	out := m.View(80, 24)
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("chain entries missing: %s", out)
	}
}

func TestExtensionsToggleListsWithCAndG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "C-only-chain", Via: "Z"}},
		Groups: []extensions.Group{
			{Name: "G-only-group", Type: "select", Proxies: []string{"DIRECT"}},
		},
	})
	m := newExtensions(path, func() []string { return nil })
	if !strings.Contains(m.View(80, 24), "C-only-chain") {
		t.Fatalf("expected chains by default")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !strings.Contains(m.View(80, 24), "G-only-group") {
		t.Fatalf("expected groups after g")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if !strings.Contains(m.View(80, 24), "C-only-chain") {
		t.Fatalf("expected chains after c")
	}
}

func TestExtensionsDeleteRemovesEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	m := newExtensions(path, func() []string { return nil })
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 0 {
		t.Fatalf("delete didn't persist: %+v", ext)
	}
}

func TestExtensionsAddChainPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	m := newExtensions(path, func() []string { return []string{"NodeA", "NodeB"} })
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !m.formOpen() {
		t.Fatalf("expected form open after a")
	}
	for _, r := range "NodeA" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "NodeB" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Node != "NodeA" || ext.Chains[0].Via != "NodeB" {
		t.Fatalf("chain not persisted: %+v", ext)
	}
}

func TestExtensionsApplyInvokesCallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	called := false
	m := newExtensions(path, func() []string { return nil })
	m.applyFunc = func() error { called = true; return nil }
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !called {
		t.Fatalf("applyFunc not invoked on r")
	}
}
