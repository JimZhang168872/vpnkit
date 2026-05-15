package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap groups global key bindings shown in the help overlay.
type KeyMap struct {
	Tab1, Tab2, Tab3, Tab4, Tab5, Tab6 key.Binding
	NextTab, PrevTab                   key.Binding
	Help, Quit                         key.Binding
	Restart, Mode                      key.Binding
	Palette                            key.Binding
}

// DefaultKeys returns the standard global key bindings.
func DefaultKeys() KeyMap {
	return KeyMap{
		Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Dashboard")),
		Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Proxies")),
		Tab3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Profiles")),
		Tab4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Connections")),
		Tab5:    key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Rules")),
		Tab6:    key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "Settings")),
		NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "next tab")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-Tab", "prev tab")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart mihomo")),
		Mode:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "cycle mode")),
		Palette: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command palette")),
	}
}
