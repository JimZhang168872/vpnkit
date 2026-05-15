// Package app contains the top-level bubbletea Model and ties subsystems together.
package app

import (
	"vpnkit/internal/msg"
)

// Re-exports so app-package consumers keep referring to TrafficMsg / VersionMsg /
// … while the canonical types live in internal/msg. Tab packages also import
// internal/msg directly, breaking the app↔dashboard cycle.
type (
	TrafficMsg           = msg.Traffic
	VersionMsg           = msg.Version
	ServiceStatusMsg     = msg.ServiceStatus
	BootstrapProgressMsg = msg.BootstrapProgress
	FlashMsg             = msg.Flash
	FlashKind            = msg.FlashKind
	TickMsg              = msg.Tick
	ProfileUpdated       = msg.ProfileUpdated
	ProfileError         = msg.ProfileError
)

const (
	FlashInfo  = msg.FlashInfo
	FlashWarn  = msg.FlashWarn
	FlashError = msg.FlashError
)

// QuitMsg signals graceful exit. Kept local because it is purely app-internal.
type QuitMsg struct{}
