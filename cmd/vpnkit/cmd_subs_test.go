package main

import (
	"bytes"
	"strings"
	"testing"

	"vpnkit/internal/store"
)

func TestSubsAddAndList(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runSubsAdd(st, "doge", "https://example.invalid/sub", ""); err != nil {
		t.Fatalf("add: %v", err)
	}
	var out bytes.Buffer
	if err := runSubsList(&out, st, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), "doge") {
		t.Errorf("list missing doge: %s", out.String())
	}
	if !strings.Contains(out.String(), "https://example.invalid/sub") {
		t.Errorf("list missing url: %s", out.String())
	}
}

func TestSubsAddDuplicate(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runSubsAdd(st, "doge", "https://example.invalid/sub", ""); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := runSubsAdd(st, "doge", "https://other.invalid/sub", "")
	if err == nil {
		t.Error("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestSubsAddEmpty(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	if err := runSubsAdd(st, "", "https://example.invalid/sub", ""); err == nil {
		t.Error("expected error for empty name")
	}
	if err := runSubsAdd(st, "doge", "", ""); err == nil {
		t.Error("expected error for empty url")
	}
}

func TestSubsRm(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2, Subscriptions: []store.Subscription{
		{Name: "doge", Enabled: true},
	}}}
	if err := runSubsRm(st, "doge"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if len(st.Cfg.Subscriptions) != 0 {
		t.Errorf("not removed: %+v", st.Cfg.Subscriptions)
	}
}

func TestSubsRmNotFound(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runSubsRm(st, "nonexistent")
	if err == nil {
		t.Error("expected error for missing subscription")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestSubsEnableDisable(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2, Subscriptions: []store.Subscription{
		{Name: "doge", Enabled: true},
	}}}
	if err := runSubsToggle(st, "doge", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if st.Cfg.Subscriptions[0].Enabled {
		t.Error("should be disabled")
	}
	if err := runSubsToggle(st, "doge", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !st.Cfg.Subscriptions[0].Enabled {
		t.Error("should be enabled")
	}
}

func TestSubsToggleNotFound(t *testing.T) {
	st := &store.Store{Cfg: store.Config{SchemaVersion: 2}}
	err := runSubsToggle(st, "nonexistent", true)
	if err == nil {
		t.Error("expected error for missing subscription")
	}
}

func TestSubsListJSON(t *testing.T) {
	st := &store.Store{Cfg: store.Config{
		SchemaVersion: 2,
		Subscriptions: []store.Subscription{
			{Name: "doge", URL: "https://example.invalid/sub", Enabled: true, NodeCount: 5},
		},
	}}
	var out bytes.Buffer
	if err := runSubsList(&out, st, true); err != nil {
		t.Fatalf("list json: %v", err)
	}
	if !strings.Contains(out.String(), `"doge"`) {
		t.Errorf("json missing doge: %s", out.String())
	}
}
