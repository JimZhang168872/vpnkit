package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRules(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rules": []map[string]any{
				{"type": "RULE-SET", "payload": "reject", "proxy": "🛑 Reject"},
				{"type": "MATCH", "payload": "", "proxy": "🚀 Proxy"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	rs, err := c.GetRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || rs[0].Type != "RULE-SET" {
		t.Errorf("got %+v", rs)
	}
}

func TestGetRuleProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"providers": map[string]any{
				"reject": map[string]any{"name": "reject", "behavior": "Domain", "ruleCount": 1234, "updatedAt": "2026-05-15T20:00:00Z"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	ps, err := c.GetRuleProviders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	p, ok := ps["reject"]
	if !ok || p.RuleCount != 1234 {
		t.Errorf("provider: %+v", p)
	}
}

func TestRefreshRuleProvider(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/providers/rules/reject" {
			called = true
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.RefreshRuleProvider(context.Background(), "reject"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("PUT not seen")
	}
}
