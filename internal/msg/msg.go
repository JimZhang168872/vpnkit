// Package msg defines tea.Msg types shared between vpnkit's app and tab packages.
// Lives in its own package so app (which composes tabs) and individual tab models
// (which consume the messages) can both depend on it without creating an import cycle.
package msg

import (
	"time"

	"vpnkit/internal/api"
)

// Traffic carries one /traffic sample.
type Traffic api.Traffic

// Version announces the mihomo version (or error) returned by /version.
type Version struct {
	Version string
	Err     error
}

// ServiceStatus snapshots the service backend status.
type ServiceStatus struct {
	Running bool
	PID     int
	Mode    string
	Since   time.Time
}

// BootstrapProgress announces a phase of the first-run flow.
type BootstrapProgress struct {
	Phase string // "downloading" | "installing-service" | "starting" | "ready"
	Note  string
	Err   error
}

// Flash is a transient status-bar notification.
type Flash struct {
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

// Tick is emitted by periodic timers.
type Tick struct{ T time.Time }

// ProxiesSnapshot is one /proxies tick from mihomo.
type ProxiesSnapshot struct {
	Groups map[string]ProxyGroup
}

// ProxyGroup is the dashboard-friendly form of an entry in /proxies.
type ProxyGroup struct {
	Name string
	Type string
	Now  string
	All  []string
}

// ProfileUpdated is dispatched when a profile finishes updating.
type ProfileUpdated struct {
	Name      string
	NodeCount int
}

// ProfileError is dispatched when a profile operation fails.
type ProfileError struct {
	Name string
	Err  error
}

// DelayResults is dispatched after a group delay test.
type DelayResults struct {
	Group   string
	Results map[string]int
}
