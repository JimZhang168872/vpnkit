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
)

// ProgressFunc reports bytes downloaded so far and the total expected (-1 if unknown).
type ProgressFunc func(n, total int64)

// Download fetches a gzipped mihomo binary, verifies SHA256 of the raw gzip stream
// against expectedSHA (hex-encoded; empty = skip check), decompresses, and writes
// the resulting executable atomically to dst with mode 0o755.
func Download(url, expectedSHA, dst string, progress ProgressFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	total := resp.ContentLength

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
	gz, err := gzip.NewReader(progressReader(reader, total, progress))
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
