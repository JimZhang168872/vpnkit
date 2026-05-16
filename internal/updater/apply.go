package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"vpnkit/internal/netx"
)

// vpnkitAssetName returns the tarball name goreleaser produces for `version`
// on the current architecture. version may be prefixed with `v`.
func vpnkitAssetName(version, arch string) string {
	v := strings.TrimPrefix(version, "v")
	return fmt.Sprintf("vpnkit_%s_linux_%s.tar.gz", v, arch)
}

// CurrentArch returns runtime arch normalized to release naming convention.
func CurrentArch() string {
	switch runtime.GOARCH {
	case "arm64", "aarch64":
		return "arm64"
	default:
		return "amd64"
	}
}

// DownloadAndApplyVpnkit fetches the .tar.gz at `githubURL` through the
// fallback chain (preferredMirror → direct → public mirrors), optionally
// verifies SHA256 of the raw tarball stream against `expectedSHA`
// (hex; empty = skip), extracts the inner `vpnkit` file, and atomically
// replaces `dstPath`. onAttempt (optional) receives each attempt's outcome.
// Returns the mirror that actually served the bytes ("" = direct).
//
// On SHA mismatch the existing binary at dstPath is left untouched.
func DownloadAndApplyVpnkit(githubURL, expectedSHA, dstPath, preferredMirror string, onAttempt netx.OnAttempt) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	body, winningMirror, err := netx.OpenWithFallback(
		ctx, githubURL, preferredMirror,
		netx.BuiltinGitHubMirrors,
		15*time.Second,
		onAttempt,
	)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", githubURL, err)
	}
	defer body.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return winningMirror, err
	}
	tmpTarball, err := os.CreateTemp(filepath.Dir(dstPath), "vpnkit-up-*.tar.gz")
	if err != nil {
		return winningMirror, err
	}
	tmpTarballName := tmpTarball.Name()
	defer os.Remove(tmpTarballName)

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpTarball, hasher), body); err != nil {
		tmpTarball.Close()
		return winningMirror, err
	}
	if err := tmpTarball.Close(); err != nil {
		return winningMirror, err
	}
	if expectedSHA != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedSHA {
			return winningMirror, fmt.Errorf("sha256 mismatch: got %s, want %s", got, expectedSHA)
		}
	}

	// Extract the inner vpnkit binary to a sibling temp file.
	tmpBinary, err := os.CreateTemp(filepath.Dir(dstPath), "vpnkit-bin-*.tmp")
	if err != nil {
		return winningMirror, err
	}
	tmpBinaryName := tmpBinary.Name()
	defer os.Remove(tmpBinaryName)

	if err := extractVpnkit(tmpTarballName, tmpBinary); err != nil {
		tmpBinary.Close()
		return winningMirror, err
	}
	if err := tmpBinary.Close(); err != nil {
		return winningMirror, err
	}
	if err := os.Chmod(tmpBinaryName, 0o755); err != nil {
		return winningMirror, err
	}

	// Atomic replace. On POSIX, rename over an executable that is currently
	// running is permitted — the running process keeps the old inode open
	// until exit; new invocations get the new binary.
	if err := os.Rename(tmpBinaryName, dstPath); err != nil {
		return winningMirror, err
	}
	return winningMirror, nil
}

// extractVpnkit walks the tarball and writes the file named "vpnkit" to w.
func extractVpnkit(tarballPath string, w io.Writer) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("tarball does not contain 'vpnkit'")
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) == "vpnkit" && h.Typeflag == tar.TypeReg {
			_, err := io.Copy(w, tr)
			return err
		}
	}
}

// parseSHA256Sums looks up the digest for `filename` in a SHA256SUMS-style
// file (lines: "<hex>  <name>"). Tolerant of trailing newlines and extra
// whitespace; returns an error if `filename` is not found.
func parseSHA256Sums(body, filename string) (string, error) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if parts[len(parts)-1] == filename {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no sha256 entry for %q", filename)
}
