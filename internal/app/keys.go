package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap groups global key bindings shown in the help overlay. Labels MUST
// match the actual tab order in TabNames (model.go) — they're shown in
// the help overlay and the sidebar advertises 7 tabs.
type KeyMap struct {
	Tab1, Tab2, Tab3, Tab4, Tab5, Tab6, Tab7 key.Binding
	NextTab, PrevTab                         key.Binding
	Help, Quit                               key.Binding
	Restart, Mode                            key.Binding
	Palette                                  key.Binding
}

// DefaultKeys returns the standard global key bindings. The Tab1..Tab7
// labels are kept in sync with TabNames in model.go so the help overlay
// doesn't lie. Pre-rc.7 the labels were stale (Tab2 said "Proxies" but
// mapped to Groups) AND Tab7 was missing entirely — pressing 7 did
// nothing despite the sidebar showing [7] Settings.
func DefaultKeys() KeyMap {
	return KeyMap{
		Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Dashboard")),
		Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Groups")),
		Tab3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Sources")),
		Tab4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Rules")),
		Tab5:    key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Connections")),
		Tab6:    key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "Logs")),
		Tab7:    key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "Settings")),
		NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "next tab")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-Tab", "prev tab")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart mihomo")),
		Mode:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "cycle mode")),
		Palette: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command palette")),
	}
}
