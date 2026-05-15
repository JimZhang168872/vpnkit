package service

import (
	"os"
	"path/filepath"
)

// Detect picks the service backend mode based on environment.
// Order: (1) $XDG_RUNTIME_DIR/systemd/private exists → systemd-user;
//       (2) systemctl --user show-environment succeeds → systemd-user;
//       (3) otherwise → pid.
// runner is the systemctl runner used in step 2; pass nil to use the real one.
func Detect(runner Runner) Mode {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		if _, err := os.Stat(filepath.Join(rt, "systemd", "private")); err == nil {
			return ModeSystemdUser
		}
	}
	if runner == nil {
		runner = defaultSystemctl
	}
	if _, err := runner("--user", "show-environment"); err == nil {
		return ModeSystemdUser
	}
	return ModePID
}

// New constructs the appropriate Manager based on Detect or an explicit mode.
// Pass an empty mode to auto-detect.
func New(mode Mode, cfg Config) Manager {
	if mode == "" {
		mode = Detect(nil)
	}
	if mode == ModeSystemdUser {
		return NewSystemd(cfg, nil)
	}
	return NewPID(cfg, nil)
}
