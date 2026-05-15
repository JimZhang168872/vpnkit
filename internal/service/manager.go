// Package service provides a non-root background-process manager for mihomo,
// with two interchangeable backends: systemd --user and a PID-file based fork.
package service

import (
	"context"
	"errors"
	"io"
	"time"
)

// Mode is the active service backend.
type Mode string

const (
	ModeSystemdUser Mode = "systemd-user"
	ModePID         Mode = "pid"
)

// Status reports the runtime state of mihomo.
type Status struct {
	Running bool
	PID     int
	Since   time.Time
	Mode    Mode
}

// ErrNotRunning is returned by Stop/Status when mihomo is not running.
var ErrNotRunning = errors.New("service: mihomo not running")

// Manager abstracts service lifecycle operations.
type Manager interface {
	Mode() Mode
	Install(ctx context.Context) error
	Uninstall(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Status(ctx context.Context) (Status, error)
	// Logs returns a reader that streams (or replays + follows) the mihomo log.
	// follow=true keeps the reader open and streams new lines; false returns the
	// last ~30 lines as a one-shot reader.
	Logs(ctx context.Context, follow bool) (io.ReadCloser, error)
}

// Config is shared by both backends.
type Config struct {
	BinaryPath  string // absolute path to mihomo binary
	ConfigDir   string // -d argument passed to mihomo
	PIDFilePath string // for PID-mode only
	LogFilePath string // for PID-mode only
	UnitPath    string // for systemd-user only
}
