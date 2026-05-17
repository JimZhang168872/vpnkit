package installer

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"vpnkit/internal/netx"
)

// ProgressFunc reports bytes downloaded so far and the total expected (-1 if unknown).
type ProgressFunc func(n, total int64)

// Download fetches a gzipped mihomo binary directly from githubURL using
// netx.SmartClient (which honors a live env proxy if one is reachable, else
// goes direct). Verifies SHA256 of the raw gzip stream against expectedSHA
// (hex; empty = skip check), decompresses, and writes the resulting
// executable atomically to dst with mode 0o755.
//
// There is no mirror fallback chain. If the GET fails, the error is returned
// as-is; callers should surface it to the user with a hint to configure a
// proxy (HTTPS_PROXY env) if they're inside a restricted network.
func Download(githubURL, expectedSHA, dst string, progress ProgressFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubURL, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", githubURL, err)
	}
	client := netx.SmartClient(0)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", githubURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %s", githubURL, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "mihomo-*.dl")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(tmpName) }

	hasher := sha256.New()
	reader := io.TeeReader(resp.Body, hasher)
	gz, err := gzip.NewReader(progressReader(reader, -1, progress))
	if err != nil {
		cleanup()
		return err
	}
	if _, err := io.Copy(tmp, gz); err != nil {
		cleanup()
		return err
	}
	if err := gz.Close(); err != nil {
		cleanup()
		return err
	}
	if expectedSHA != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedSHA {
			cleanup()
			return fmt.Errorf("sha256 mismatch: got %s expected %s", got, expectedSHA)
		}
	}
	if err := tmp.Chmod(0o755); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

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
