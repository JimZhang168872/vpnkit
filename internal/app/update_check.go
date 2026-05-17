package app

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/updater"
)

// UpdateAvailableMsg surfaces a successful (no-error) update check.
// Sent at most once per session, never sent if nothing is upgradable or
// the version string says we're a dev build.
type UpdateAvailableMsg struct {
	Info updater.Info
}

// pollUpdate runs a single GitHub release check in the background. Errors
// are swallowed — a missing network shouldn't surface a flash to the user.
// We delay 2s so the bootstrap status messages have time to settle before
// any "⚡ update available" line shows up.
func pollUpdate(prog *tea.Program, vpnkitVer, mihomoBinary string) {
	time.Sleep(2 * time.Second)
	mihomoVer := readMihomoVersionForCheck(mihomoBinary)
	info, err := updater.Check(updater.Opts{
		VpnkitCurrent: vpnkitVer,
		MihomoCurrent: mihomoVer,
	})
	if err != nil {
		return
	}
	if !info.HasUpdate() {
		return
	}
	prog.Send(UpdateAvailableMsg{Info: info})
}

func readMihomoVersionForCheck(binary string) string {
	if _, err := os.Stat(binary); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v").Output()
	if err != nil {
		return ""
	}
	s := string(out)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "v") && len(f) > 1 && (f[1] >= '0' && f[1] <= '9') {
			return f
		}
	}
	return ""
}
