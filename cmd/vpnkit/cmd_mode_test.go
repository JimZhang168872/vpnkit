package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
	"vpnkit/internal/store"
)

// mockStoreLoad overrides storeLoad for mode tests that don't need a real store.
func withMockStore(t *testing.T, mode string) func() {
	t.Helper()
	origLoad := storeLoad
	storeLoad = func(path string) (*store.Store, error) {
		return &store.Store{Cfg: store.Config{
			SchemaVersion: 2,
			Mode:          mode,
		}}, nil
	}
	return func() { storeLoad = origLoad }
}

func TestModeShow(t *testing.T) {
	// mode show reads from store, not controller.
	restore := withMockStore(t, "rule")
	defer restore()

	// server is needed because runMode requires a client (even for show, the
	// constructor doesn't fail — we just don't use the controller for show).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := api.New(srv.URL, "")

	var buf bytes.Buffer
	if err := runMode(&buf, c, nil, false); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "rule" {
		t.Errorf("got %q, want %q", buf.String(), "rule")
	}
}

func TestModeShowJSON(t *testing.T) {
	restore := withMockStore(t, "global")
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := api.New(srv.URL, "")

	var buf bytes.Buffer
	if err := runMode(&buf, c, nil, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["mode"] != "global" {
		t.Errorf("mode: %v", got["mode"])
	}
}

func TestModeSet(t *testing.T) {
	// For mode set, we need a real store on disk + a controller that accepts PUT.
	p, restoreEnv := initEnv(t)
	defer restoreEnv()

	var initBuf bytes.Buffer
	if err := runInit(&initBuf, runInitOpts{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Reset storeLoad to use real store.
	origLoad := storeLoad
	storeLoad = store.Load
	defer func() { storeLoad = origLoad }()

	calls := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		// ReloadConfig uses PUT.
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")

	var buf bytes.Buffer
	if err := runMode(&buf, c, []string{"global"}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "→ global") {
		t.Errorf("output: %q", buf.String())
	}
	// Verify store was updated.
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if st.Cfg.Mode != "global" {
		t.Errorf("store mode: %q", st.Cfg.Mode)
	}
	// Verify reload was called (PUT /configs).
	found := false
	for _, m := range calls {
		if m == http.MethodPut {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PUT /configs call, got: %v", calls)
	}
}

func TestModeInvalid(t *testing.T) {
	restore := withMockStore(t, "rule")
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := api.New(srv.URL, "")

	var buf bytes.Buffer
	err := runMode(&buf, c, []string{"foobar"}, false)
	if err == nil || !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("expected invalid-mode error, got %v", err)
	}
}
