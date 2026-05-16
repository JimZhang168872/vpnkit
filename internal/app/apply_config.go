package app

import (
	"context"
	"errors"
	"fmt"

	"vpnkit/internal/service"
)

// configReloader is the subset of api.Client we need for hot-reloading mihomo.
// Defined as a small interface so the apply-config logic can be unit-tested
// with a fake instead of spinning up an HTTP server.
type configReloader interface {
	ReloadConfig(ctx context.Context, path string) error
}

// applyConfig nudges mihomo to pick up whatever config.yaml is on disk.
//
// It first tries a hot reload via mihomo's `/configs` PUT (fast, no proxy
// interruption). If that fails — most commonly because vpnkit's stored
// controller_secret has drifted from the one mihomo loaded into memory at
// boot, making every controller API call 401 — it falls back to a full
// service restart. A restart re-reads config.yaml from disk and resyncs
// secret + auth + proxies + ports in one shot.
//
// Returns the restart error if both paths fail. Returns the reload error if
// reload failed and no service manager is available to fall back to.
func applyConfig(ctx context.Context, r configReloader, svc service.Manager) error {
	if r == nil {
		return errors.New("applyConfig: nil reloader")
	}
	if err := r.ReloadConfig(ctx, ""); err == nil {
		return nil
	} else if svc == nil {
		return fmt.Errorf("mihomo reload failed and no service handle to restart: %w", err)
	}
	if err := svc.Restart(ctx); err != nil {
		return fmt.Errorf("mihomo restart fallback failed: %w", err)
	}
	return nil
}
