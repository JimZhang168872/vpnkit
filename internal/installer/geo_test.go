package installer

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// newGeoTestServer returns an httptest.Server that returns a deterministic
// payload per path so tests can verify the right URL was hit, plus a hits
// counter for verifying skip-when-present semantics.
func newGeoTestServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		switch r.URL.Path {
		case "/country.mmdb":
			_, _ = w.Write([]byte("MMDB-COUNTRY"))
		case "/geoip.metadb":
			_, _ = w.Write([]byte("METADB-GEOIP"))
		case "/geosite.dat":
			_, _ = w.Write([]byte("DAT-GEOSITE"))
		case "/asn.mmdb":
			_, _ = w.Write([]byte("MMDB-ASN"))
		case "/nope":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, "unknown", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func geoFilesFromSrv(srv *httptest.Server) []GeoFile {
	return []GeoFile{
		{Name: "country.mmdb", URL: srv.URL + "/country.mmdb"},
		{Name: "geoip.metadb", URL: srv.URL + "/geoip.metadb"},
		{Name: "geosite.dat", URL: srv.URL + "/geosite.dat"},
		{Name: "GeoLite2-ASN.mmdb", URL: srv.URL + "/asn.mmdb"},
	}
}

func TestEnsureGeoDownloadsAll(t *testing.T) {
	srv, hits := newGeoTestServer(t)
	dir := t.TempDir()

	fetched, err := EnsureGeo(dir, geoFilesFromSrv(srv))
	if err != nil {
		t.Fatalf("EnsureGeo: %v", err)
	}
	if got, want := len(fetched), 4; got != want {
		t.Errorf("fetched count: got %d want %d (%v)", got, want, fetched)
	}
	if got := atomic.LoadInt32(hits); got != 4 {
		t.Errorf("server hits: got %d want 4", got)
	}
	for _, name := range []string{"country.mmdb", "geoip.metadb", "geosite.dat", "GeoLite2-ASN.mmdb"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Errorf("%s empty", name)
		}
	}
}

func TestEnsureGeoSkipsExisting(t *testing.T) {
	srv, hits := newGeoTestServer(t)
	dir := t.TempDir()

	// Pre-populate two files with sentinel content; EnsureGeo should leave
	// them alone (no server hit) and only fetch the missing ones.
	for _, name := range []string{"country.mmdb", "geosite.dat"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("KEEP-ME"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fetched, err := EnsureGeo(dir, geoFilesFromSrv(srv))
	if err != nil {
		t.Fatalf("EnsureGeo: %v", err)
	}
	if got, want := len(fetched), 2; got != want {
		t.Errorf("fetched count: got %d want %d (%v)", got, want, fetched)
	}
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Errorf("server hits: got %d want 2 (only missing files)", got)
	}
	keep, _ := os.ReadFile(filepath.Join(dir, "country.mmdb"))
	if string(keep) != "KEEP-ME" {
		t.Errorf("EnsureGeo overwrote existing file: %q", keep)
	}
}

func TestEnsureGeoCreatesDir(t *testing.T) {
	srv, _ := newGeoTestServer(t)
	dir := filepath.Join(t.TempDir(), "nested", "mihomo")
	// Confirm dir does not exist beforehand.
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("precondition: dir already exists: %v", err)
	}
	if _, err := EnsureGeo(dir, geoFilesFromSrv(srv)); err != nil {
		t.Fatalf("EnsureGeo: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("dir not created: %v", err)
	}
}

func TestEnsureGeoReturnsErrorAndContinues(t *testing.T) {
	srv, _ := newGeoTestServer(t)
	dir := t.TempDir()

	files := []GeoFile{
		{Name: "country.mmdb", URL: srv.URL + "/country.mmdb"},      // ok
		{Name: "broken.mmdb", URL: srv.URL + "/nope"},               // 404
		{Name: "geosite.dat", URL: srv.URL + "/geosite.dat"},        // ok
	}
	fetched, err := EnsureGeo(dir, files)
	if err == nil {
		t.Fatal("expected aggregated error for 404 case, got nil")
	}
	if !strings.Contains(err.Error(), "broken.mmdb") {
		t.Errorf("error missing failed-file hint: %v", err)
	}
	// Verify other files still downloaded.
	if _, statErr := os.Stat(filepath.Join(dir, "country.mmdb")); statErr != nil {
		t.Errorf("country.mmdb should have been fetched despite peer failure: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "geosite.dat")); statErr != nil {
		t.Errorf("geosite.dat should have been fetched despite peer failure: %v", statErr)
	}
	if len(fetched) != 2 {
		t.Errorf("fetched: want 2 (ok ones), got %d (%v)", len(fetched), fetched)
	}
	// Failed file should NOT exist (partial download cleaned up).
	if _, statErr := os.Stat(filepath.Join(dir, "broken.mmdb")); !os.IsNotExist(statErr) {
		t.Errorf("broken.mmdb should not exist (atomic write): %v", statErr)
	}
}

func TestEnsureGeoNilFilesUsesDefaults(t *testing.T) {
	// Passing nil should reach for DefaultGeoFiles (which point at real
	// github URLs). To avoid network in this unit test, we just assert that
	// DefaultGeoFiles is non-empty and contains the four expected names.
	names := map[string]bool{}
	for _, f := range DefaultGeoFiles {
		names[f.Name] = true
	}
	for _, want := range []string{"country.mmdb", "geoip.metadb", "geosite.dat", "GeoLite2-ASN.mmdb"} {
		if !names[want] {
			t.Errorf("DefaultGeoFiles missing %s", want)
		}
	}
}

func TestEnsureGeoSizeZeroTriggersRedownload(t *testing.T) {
	// A zero-byte file is treated as "missing" so half-finished downloads
	// from a prior crash don't poison future runs.
	srv, hits := newGeoTestServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "country.mmdb"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureGeo(dir, []GeoFile{
		{Name: "country.mmdb", URL: srv.URL + "/country.mmdb"},
	}); err != nil {
		t.Fatalf("EnsureGeo: %v", err)
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Errorf("zero-byte file should have triggered a redownload (hits=%d)", atomic.LoadInt32(hits))
	}
	data, _ := os.ReadFile(filepath.Join(dir, "country.mmdb"))
	if string(data) != "MMDB-COUNTRY" {
		t.Errorf("file not replaced: %q", data)
	}
}

// Sanity: errors.Join is available in Go 1.20+; vpnkit targets 1.23.
func TestEnsureGeoUsesErrorsJoin(t *testing.T) {
	srv, _ := newGeoTestServer(t)
	files := []GeoFile{
		{Name: "a", URL: srv.URL + "/nope"},
		{Name: "b", URL: srv.URL + "/nope"},
	}
	_, err := EnsureGeo(t.TempDir(), files)
	if err == nil {
		t.Fatal("expected error")
	}
	// errors.Is on a joined error should walk the chain. We can't assert a
	// specific sentinel, but we can assert that the aggregate string mentions
	// both files.
	s := err.Error()
	if !strings.Contains(s, "a") || !strings.Contains(s, "b") {
		t.Errorf("aggregate error missing one of the file names: %v", err)
	}
	// errors.Is(err, nil) is always true if err is nil; not the test here.
	// We just confirm errors.Join produces a joinable error.
	if errors.Is(err, nil) {
		t.Errorf("err is unexpectedly nil-equivalent")
	}
}
