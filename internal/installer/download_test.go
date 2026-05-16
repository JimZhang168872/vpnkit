package installer

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vpnkit/internal/netx"
)

// Clear proxy env leaks from the parent shell — SmartClient inside Download
// would otherwise route httptest URLs through a still-listening mihomo on
// 127.0.0.1:7890, garbling the test fixtures.
func init() {
	for _, k := range []string{
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
		"http_proxy", "https_proxy", "all_proxy",
	} {
		_ = os.Unsetenv(k)
	}
}

func TestDownloadAndVerify(t *testing.T) {
	payload := []byte("hello mihomo")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(payload)
	_ = gw.Close()
	gzBytes := buf.Bytes()
	sum := sha256.Sum256(gzBytes)
	expected := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzBytes)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "mihomo")
	progresses := []int64{}
	_, err := Download(srv.URL+"/mihomo.gz", expected, dst, "", nil, func(n, total int64) {
		progresses = append(progresses, n)
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch: %q", got)
	}
	if len(progresses) == 0 {
		t.Errorf("progress callback never fired")
	}
	info, _ := os.Stat(dst)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("perm: %v", info.Mode().Perm())
	}
}

func TestDownloadSHAMismatch(t *testing.T) {
	gzBytes, _ := gzipBytes([]byte("payload"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gzBytes)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "mihomo")
	_, err := Download(srv.URL, "00deadbeef", dst, "", nil, nil)
	if err == nil {
		t.Fatal("expected SHA mismatch error")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("partial file left behind: %v", err)
	}
}

func gzipBytes(p []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(p); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestDownloadHTTPError(t *testing.T) {
	// Skip the public-mirror chain in this test — it would otherwise spend
	// 45 s trying ghproxy.com/etc. wrapped around the httptest URL.
	saved := netx.BuiltinGitHubMirrors
	netx.BuiltinGitHubMirrors = nil
	defer func() { netx.BuiltinGitHubMirrors = saved }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "mihomo")
	if _, err := Download(srv.URL, "", dst, "", nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

var _ = io.EOF
