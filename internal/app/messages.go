// Package app contains the top-level bubbletea Model and ties subsystems together.
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
)

// TrafficMsg carries one /traffic sample.
type TrafficMsg api.Traffic

// VersionMsg announces the mihomo version (or error) returned by /version.
type VersionMsg struct {
	Version string
	Err     error
}

// ServiceStatusMsg snapshots the service backend status.
type ServiceStatusMsg struct {
	Running bool
	PID     int
	Mode    string
	Since   time.Time
}

// BootstrapProgressMsg announces a phase of the first-run flow.
type BootstrapProgressMsg struct {
	Phase string // "downloading" | "installing-service" | "starting" | "ready"
	Note  string
	Err   error
}

// FlashMsg is a transient status-bar notification.
type FlashMsg struct {
	Text  string
	Kind  FlashKind
	Until time.Time
}

// FlashKind distinguishes severity for styling.
type FlashKind int

const (
	FlashInfo FlashKind = iota
	FlashWarn
	FlashError
)

// TickMsg is emitted by periodic timers.
type TickMsg struct{ T time.Time }

// QuitMsg signals graceful exit.
type QuitMsg struct{}

// Compile-time interface checks.
var (
	_ tea.Msg = TrafficMsg{}
	_ tea.Msg = VersionMsg{}
)
