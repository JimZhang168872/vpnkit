package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestModeShow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": 7890})
	}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runMode(&buf, c, nil, false); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "rule" {
		t.Errorf("got %q", buf.String())
	}
}

func TestModeSet(t *testing.T) {
	calls := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule"})
		case http.MethodPatch:
			// no body needed in response
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runMode(&buf, c, []string{"global"}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "rule → global") {
		t.Errorf("output: %s", buf.String())
	}
	if len(calls) != 2 || calls[0] != http.MethodGet || calls[1] != http.MethodPatch {
		t.Errorf("calls: %v", calls)
	}
}

func TestModeInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	err := runMode(&buf, c, []string{"foobar"}, false)
	if err == nil || !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("expected invalid-mode error, got %v", err)
	}
}
