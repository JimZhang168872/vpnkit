package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"vpnkit/internal/api"
)

func TestIPHuman(t *testing.T) {
	// Mock ipinfo.io
	ipsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ip": "203.0.113.42", "country": "HK", "region": "Central and Western",
			"city": "Hong Kong", "org": "AS12345 Example Hosting Ltd.",
		})
	}))
	defer ipsrv.Close()
	// Mock mihomo /configs (returns mixed-port that points BACK to ipsrv so the
	// "proxy fetch" actually succeeds without a real proxy in the loop)
	ipURL, _ := url.Parse(ipsrv.URL)
	port, _ := strconv.Atoi(ipURL.Port())

	mux := http.NewServeMux()
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": "rule", "mixed-port": port})
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"🚀 Proxy": map[string]any{"type": "Selector", "now": "HK-01", "all": []string{"HK-01"}},
			},
		})
	})
	mihomoSrv := httptest.NewServer(mux)
	defer mihomoSrv.Close()

	c := api.New(mihomoSrv.URL, "")
	var buf bytes.Buffer
	if err := runIP(&buf, c, nil, ipsrv.URL+"/json", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"203.0.113.42", "HK", "Hong Kong", "AS12345", "🚀 Proxy"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPMihomoUnreachable(t *testing.T) {
	c := api.New("http://127.0.0.1:1", "")
	var buf bytes.Buffer
	err := runIP(&buf, c, nil, "https://ipinfo.io/json", false)
	if err == nil {
		t.Error("expected error")
	}
}
