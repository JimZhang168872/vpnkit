package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vpnkit/internal/api"
	"vpnkit/internal/store"
)

func TestStatusHumanOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.19.16", "meta": true})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": 7890})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"DIRECT":   map[string]any{"type": "Direct"},
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01", "JP-02"}},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runStatus(&buf, c, nil, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"v1.19.16", "running", "rule", "mixed=7890", "🚀 Proxy", "HK-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusJSONOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.0.0", "meta": true})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "global", "mixed-port": 7891})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runStatus(&buf, c, nil, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["mode"] != "global" {
		t.Errorf("mode: %v", got["mode"])
	}
}

func TestStatusUnreachable(t *testing.T) {
	c := api.New("http://127.0.0.1:1", "")
	c.HTTP.Timeout = 200 * time.Millisecond
	var buf bytes.Buffer
	err := runStatus(&buf, c, nil, false)
	if err == nil {
		t.Error("expected error for unreachable mihomo")
	}
	_ = context.Canceled
}

func TestStatusWithStore(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.19.16", "meta": true})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": 7890})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := api.New(srv.URL, "")
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		Mode:          "rule",
		GlobalTarget:  "doge-auto",
		Subscriptions: []store.Subscription{
			{Name: "doge", URL: "https://example.invalid/sub", Enabled: true},
			{Name: "boost", URL: "https://example.invalid/boost", Enabled: false},
		},
		LocalNodes: []store.LocalNode{
			{Name: "HK-Manual", Proto: "hysteria2", Server: "1.2.3.4", Port: 443},
		},
	}}

	var buf bytes.Buffer
	if err := runStatus(&buf, c, st, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"2 subs", "1 local nodes", "mode=rule", "target=doge-auto"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestStatusWithStoreJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "v1.0.0", "meta": true})
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": 7890})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := api.New(srv.URL, "")
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		Mode:          "global",
		GlobalTarget:  "my-group",
		Subscriptions: []store.Subscription{{Name: "doge", Enabled: true}},
	}}

	var buf bytes.Buffer
	if err := runStatus(&buf, c, st, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if subs, _ := got["subscriptions"].(float64); int(subs) != 1 {
		t.Errorf("subscriptions: %v", got["subscriptions"])
	}
	if got["global_target"] != "my-group" {
		t.Errorf("global_target: %v", got["global_target"])
	}
}
