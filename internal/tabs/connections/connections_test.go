package connections

import (
	"strings"
	"testing"

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
