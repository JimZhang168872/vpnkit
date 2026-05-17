package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func loadStoreTOML(t *testing.T, iso *isoEnv) map[string]any {
	t.Helper()
	path := filepath.Join(iso.home, ".config", "vpnkit", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	return m
}

func TestTUILocalNodesAddViaURI(t *testing.T) {
	iso := newIsolatedHome(t)
	sess := newTUISession(t, iso)
	// Sources tab → Right (FocusContent) → Down (LocalNodes sub-page).
	sess.SendKeys("Left", "Down", "Down", "Right", "Down")
	// First need a group: press N, wait for the group-name form to appear.
	sess.SendKeys("N")
	sess.WaitFor("New Local Group", 3*time.Second)
	sess.SendLiteral("home")
	sess.SendKeys("Enter")
	// Wait for group to be created before opening the URI form.
	sess.WaitFor("home", 3*time.Second)
	// Open URI form via 'u' from list.
	sess.SendKeys("u")
	sess.WaitFor("Add Local Node", 3*time.Second)
	sess.SendLiteral("ss://YWVzLTI1Ni1nY206TXlQYXNzMTIz@1.2.3.4:8388#JP-test")
	sess.SendKeys("Enter")
	if !strings.Contains(sess.Capture(), "JP-test") {
		t.Errorf("JP-test not visible:\n%s", sess.Capture())
	}
}

func TestTUINewLocalGroup(t *testing.T) {
	iso := newIsolatedHome(t)
	sess := newTUISession(t, iso)
	sess.SendKeys("Left", "Down", "Down", "Right", "Down")
	// Press N, wait for the group-name form to render before typing.
	sess.SendKeys("N")
	sess.WaitFor("New Local Group", 3*time.Second)
	sess.SendLiteral("home")
	sess.SendKeys("Enter")
	sess.WaitFor("home", 3*time.Second)
	sess.MustContain("home")
	// Verify it landed in store.toml.
	st := loadStoreTOML(t, iso)
	rawGroups, _ := st["local_node_groups"].([]map[string]any)
	if len(rawGroups) != 1 || rawGroups[0]["name"] != "home" {
		t.Errorf("expected [home] group in store, got %+v", st["local_node_groups"])
	}
}
