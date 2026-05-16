package rules

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
)

// RefreshAllProvidersCmd builds a tea.Cmd that triggers refresh on every
// rule-provider in `names`. Returns nil msg (refresh result is observed via
// the next /providers/rules poll).
func RefreshAllProvidersCmd(client *api.Client, names []string) tea.Cmd {
	return func() tea.Msg {
		for _, name := range names {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = client.RefreshRuleProvider(ctx, name)
			cancel()
		}
		return nil
	}
}
