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

// TestRunTestSingleNode verifies a single-node delay test hits
// /proxies/<node>/delay and renders "<name>  XXX ms".
func TestRunTestSingleNode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxies/HK-01/delay", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") != "https://example.com/probe" {
			t.Errorf("test url not forwarded: %q", r.URL.Query().Get("url"))
		}
		if r.URL.Query().Get("timeout") != "3000" {
			t.Errorf("timeout not forwarded: %q", r.URL.Query().Get("timeout"))
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"delay": 234})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runTest(&buf, c, "", "HK-01", "https://example.com/probe", 3000, false); err != nil {
		t.Fatalf("runTest: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "HK-01") || !strings.Contains(out, "234 ms") {
		t.Errorf("output missing node+delay:\n%s", out)
	}
}

// TestRunTestGroup verifies that runTest probes the vpnkit url-test
// companion group (<name>-auto) first, which is the happy path for every
// subscription / local-nodes group emitted by the assembler.
func TestRunTestGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/group/doge-auto/delay", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{
			"HK-01": 234,
			"JP-02": 567,
			"US-03": 0, // timeout
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runTest(&buf, c, "doge", "", "https://www.gstatic.com/generate_204", 5000, false); err != nil {
		t.Fatalf("runTest: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"HK-01", "234 ms", "JP-02", "567 ms", "US-03", "timeout"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestRunTestJSON checks --json prints a structured map of node→delay (ms);
// 0 stays as 0 (caller can interpret as timeout).
func TestRunTestJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/group/doge-auto/delay", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"HK-01": 234, "JP-02": 0})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := api.New(srv.URL, "")
	var buf bytes.Buffer
	if err := runTest(&buf, c, "doge", "", "https://www.gstatic.com/generate_204", 5000, true); err != nil {
		t.Fatalf("runTest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not valid json: %v\n%s", err, buf.String())
	}
	if got["group"] != "doge" {
		t.Errorf("group field: %v", got["group"])
	}
	results, ok := got["results"].(map[string]any)
	if !ok {
		t.Fatalf("results field missing: %v", got)
	}
	if results["HK-01"] != float64(234) {
		t.Errorf("HK-01 delay: %v", results["HK-01"])
	}
	if results["JP-02"] != float64(0) {
		t.Errorf("JP-02 delay should stay 0 in JSON: %v", results["JP-02"])
	}
}

// TestRunTestMissingArgsRejected ensures runTest errors when both group and
// node are blank (caller should reject earlier via dispatchTest, but the
// helper should still validate).
func TestRunTestMissingArgs(t *testing.T) {
	c := api.New("http://127.0.0.1:1", "")
	var buf bytes.Buffer
	if err := runTest(&buf, c, "", "", "https://x", 1000, false); err == nil {
		t.Error("expected error for missing group+node")
	}
}
