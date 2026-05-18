package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestMeasureGroup_DirectGroupDelaySucceeds covers the happy path where the
// group is url-test / fallback / load-balance and mihomo accepts
// /group/<name>/delay directly.
func TestMeasureGroup_DirectGroupDelaySucceeds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/group/doge-auto/delay", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{
			"doge:HK-01": 234,
			"doge:JP-02": 0,
		})
	})
	mux.HandleFunc("/group/doge/delay", func(w http.ResponseWriter, r *http.Request) {
		// Should NOT be called — measureGroup is supposed to test the auto
		// variant when caller passes "doge". Fail loudly if hit.
		t.Errorf("/group/doge/delay unexpectedly called")
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "")

	results, err := c.MeasureGroup(context.Background(), "doge", "https://x", 5000)
	if err != nil {
		t.Fatalf("MeasureGroup: %v", err)
	}
	if results["doge:HK-01"] != 234 || results["doge:JP-02"] != 0 {
		t.Errorf("results: %v", results)
	}
}

// TestMeasureGroup_FallsBackOnSelectorGroup covers the case where neither
// /group/<name>/delay nor /group/<name>-auto/delay exist on mihomo — the
// helper must fetch /proxies to enumerate members, then call
// /proxies/<member>/delay for each in parallel.
func TestMeasureGroup_FallsBackOnSelectorGroup(t *testing.T) {
	var perNodeHits sync.Map
	mux := http.NewServeMux()
	mux.HandleFunc("/group/", func(w http.ResponseWriter, r *http.Request) {
		// Both /group/local/delay and /group/local-auto/delay must 404.
		http.Error(w, `{"message":"Resource not found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"local": map[string]any{
					"type": "Selector",
					"now":  "local:JP-A",
					"all":  []string{"local:JP-A", "local:JP-B"},
				},
			},
		})
	})
	mux.HandleFunc("/proxies/", func(w http.ResponseWriter, r *http.Request) {
		// Member delay endpoint: /proxies/<urlescaped-name>/delay
		name := strings.TrimPrefix(r.URL.Path, "/proxies/")
		name = strings.TrimSuffix(name, "/delay")
		perNodeHits.Store(name, true)
		var d int
		switch name {
		case "local:JP-A":
			d = 123
		case "local:JP-B":
			d = 0 // simulate timeout
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"delay": d})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "")

	results, err := c.MeasureGroup(context.Background(), "local", "https://x", 5000)
	if err != nil {
		t.Fatalf("MeasureGroup: %v", err)
	}
	if got := results["local:JP-A"]; got != 123 {
		t.Errorf("local:JP-A delay = %d, want 123", got)
	}
	if got, ok := results["local:JP-B"]; !ok || got != 0 {
		t.Errorf("local:JP-B should be present with 0 (timeout), got %d ok=%v", got, ok)
	}
	for _, want := range []string{"local:JP-A", "local:JP-B"} {
		if _, ok := perNodeHits.Load(want); !ok {
			t.Errorf("expected per-node delay call for %q", want)
		}
	}
}

// TestIsUnreachable detects mihomo-down errors (connection refused, no
// route, dial timeout) so callers can auto-recover via service restart.
// All these strings appear on Linux when mihomo isn't listening:
//   - "connection refused"          — process not running
//   - "no route to host"            — interface down (rare on localhost)
//   - "context deadline exceeded"   — dial timed out (slow startup mid-flight)
//   - "EOF"                         — process died mid-request
func TestIsUnreachable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{&HTTPError{StatusCode: 404}, false},
		{&HTTPError{StatusCode: 401}, false},
		{&HTTPError{StatusCode: 500}, false},
		{errStub("dial tcp 127.0.0.1:38696: connect: connection refused"), true},
		{errStub("dial tcp: connect: no route to host"), true},
		{errStub("net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"), true},
		{errStub("read tcp: EOF"), true},
		{errStub("some other random error"), false},
	}
	for _, c := range cases {
		if got := IsUnreachable(c.err); got != c.want {
			t.Errorf("IsUnreachable(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

// TestMeasureGroup_PreservesOtherErrors verifies that non-404 errors from
// the /group endpoint surface as-is (not silently translated to per-node
// fallback). 401 / 500 etc. should reach the caller.
func TestMeasureGroup_PreservesOtherErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/group/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "")

	if _, err := c.MeasureGroup(context.Background(), "doge", "https://x", 5000); err == nil {
		t.Error("expected error on 401, got nil")
	}
}
