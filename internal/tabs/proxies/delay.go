package proxies

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/msg"
)

// DelayCmd returns a tea.Cmd that performs a group delay test and emits
// msg.DelayResults on completion (or empty results on error).
func DelayCmd(client *api.Client, group string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := client.GroupDelay(ctx, group, "https://www.gstatic.com/generate_204", 5000)
		if err != nil {
			return msg.DelayResults{Group: group, Results: map[string]int{}}
		}
		return msg.DelayResults{Group: group, Results: res}
	}
}
