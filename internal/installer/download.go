package installer

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"vpnkit/internal/netx"
)

// ProgressFunc reports bytes downloaded so far and the total expected (-1 if unknown).
type ProgressFunc func(n, total int64)

// Download fetches a gzipped mihomo binary through a fallback chain (preferred
// mirror → direct → builtin public mirrors), verifies SHA256 of the raw gzip
// stream against expectedSHA (hex; empty = skip check), decompresses, and
// writes the resulting executable atomically to dst with mode 0o755.
//
// Returns the mirror that actually served the bytes (empty = direct github,
// or one of netx.BuiltinGitHubMirrors). Callers should persist a non-empty
// winner so the next download skips the dead-direct timeout.
func Download(githubURL, expectedSHA, dst, preferredMirror string, progress ProgressFunc) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	body, winningMirror, err := netx.OpenWithFallback(
		ctx, githubURL, preferredMirror,
		netx.BuiltinGitHubMirrors,
		15*time.Second, // per-attempt connect/read timeout
	)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", githubURL, err)
	}
	defer body.Close()

	total := int64(-1) // chunked / unknown via fallback path

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return winningMirror, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "mihomo-*.dl")
	if err != nil {
		return winningMirror, err
	}
	tmpName := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(tmpName) }

	hasher := sha256.New()
	reader := io.TeeReader(body, hasher)
	gz, err := gzip.NewReader(progressReader(reader, total, progress))
	if err != nil {
		cleanup()
		return winningMirror, err
	}
	if _, err := io.Copy(tmp, gz); err != nil {
		cleanup()
		return winningMirror, err
	}
	if err := gz.Close(); err != nil {
		cleanup()
		return winningMirror, err
	}
	if expectedSHA != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedSHA {
			cleanup()
			return winningMirror, fmt.Errorf("sha256 mismatch: got %s expected %s", got, expectedSHA)
		}
	}
	if err := tmp.Chmod(0o755); err != nil {
		cleanup()
		return winningMirror, err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return winningMirror, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return winningMirror, err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return winningMirror, err
	}
	return winningMirror, nil
}

// progressReader wraps r, invoking cb at most every 64KiB.
func progressReader(r io.Reader, total int64, cb ProgressFunc) io.Reader {
	if cb == nil {
		return r
	}
	return &progressR{r: r, total: total, cb: cb}
}

type progressR struct {
	r        io.Reader
	total    int64
	read     int64
	cb       ProgressFunc
	lastEmit int64
}

func (p *progressR) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.read-p.lastEmit > 64*1024 || err == io.EOF {
		p.cb(p.read, p.total)
		p.lastEmit = p.read
	}
	if errors.Is(err, io.EOF) {
		return n, err
	}
	return n, err
}
