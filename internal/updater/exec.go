package updater

import (
	"fmt"
	"os"
	"syscall"
)

// ExecSelf replaces the current process with a fresh invocation of the same
// binary path (now pointing at the just-updated executable). All open file
// descriptors and the current env are preserved by the kernel exec.
//
// This is Linux-only; vpnkit doesn't target other platforms. On any other
// OS this returns an error so the caller can degrade gracefully.
func ExecSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	// /proc/self/exe still points at the old inode after rename, so resolve
	// the symlink to the real path which is now the new binary.
	if resolved, err := os.Readlink(exe); err == nil && resolved != "" {
		exe = resolved
	}
	argv := append([]string{exe}, os.Args[1:]...)
	return syscall.Exec(exe, argv, os.Environ())
}
