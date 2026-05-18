package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Lock is a POSIX advisory lock held on a `<config>.lock` file next to
// the store. CLI mutation dispatchers acquire this before Load+mutate+Save
// so two concurrent `vpnkit subs add &` workers can't race read-modify-
// write each other.
//
// Without it, the short-lived CLI process is purely single-process-safe
// (Store.mu) but inter-process-unsafe — the QA harness confirmed 20
// parallel `subs add` calls dropped 18 entries silently.
type Lock struct {
	f *os.File
}

// AcquireLock opens (or creates) `<cfgPath>.lock` and takes an exclusive
// flock. Blocks until the lock is available. Returns a Lock the caller
// MUST Release() — typically via defer right after acquisition.
//
// Failure modes worth surfacing: parent dir doesn't exist (NotExist),
// permission denied (Permission). Both wrap the underlying error.
func AcquireLock(cfgPath string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return nil, fmt.Errorf("store lock dir: %w", err)
	}
	f, err := os.OpenFile(cfgPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("store lock open: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("store lock acquire: %w", err)
	}
	return &Lock{f: f}, nil
}

// Release drops the advisory lock and closes the file. Idempotent.
func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
	l.f = nil
}
