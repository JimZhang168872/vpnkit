package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProxies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"GLOBAL": map[string]any{"type": "Selector", "now": "DIRECT", "all": []string{"DIRECT", "REJECT"}},
				"DIRECT": map[string]any{"type": "Direct"},
				"HK-01": map[string]any{
					"type": "Shadowsocks",
					"history": []map[string]any{
						{"time": "2026-05-16T10:00:00Z", "delay": 45},
						{"time": "2026-05-16T10:01:00Z", "delay": 47},
					},
				},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	out, err := c.GetProxies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	g, ok := out["GLOBAL"]
	if !ok || g.Type != "Selector" || g.Now != "DIRECT" {
		t.Errorf("GLOBAL: %+v", g)
	}
	if len(g.All) != 2 {
		t.Errorf("All: %v", g.All)
	}
	hk, ok := out["HK-01"]
	if !ok {
		t.Fatal("HK-01 missing")
	}
	if len(hk.History) != 2 || hk.History[1].Delay != 47 {
		t.Errorf("HK-01 history: %+v", hk.History)
	}
}

func TestPutProxy(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.PutProxy(context.Background(), "🚀 Proxy", "HK-01"); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "HK-01" {
		t.Errorf("body: %v", got)
	}
}

func TestDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"delay": 123})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	d, err := c.Delay(context.Background(), "HK-01", "https://www.gstatic.com/generate_204", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if d != 123 {
		t.Errorf("delay: %d", d)
	}
}
