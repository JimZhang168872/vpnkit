package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/extensions"
)

func TestRunChainLsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	if err := runChainLs(&buf, path, false); err != nil {
		t.Fatalf("runChainLs: %v", err)
	}
	if !strings.Contains(buf.String(), "no chains") {
		t.Fatalf("expected 'no chains' line, got %q", buf.String())
	}
}

func TestRunChainSetAdds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	if err := runChainSet(&buf, path, "NodeA", "NodeB"); err != nil {
		t.Fatalf("runChainSet: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Node != "NodeA" || ext.Chains[0].Via != "NodeB" {
		t.Fatalf("not persisted: %+v", ext)
	}
}

func TestRunChainSetUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	var buf bytes.Buffer
	if err := runChainSet(&buf, path, "A", "C"); err != nil {
		t.Fatalf("runChainSet: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Via != "C" {
		t.Fatalf("not updated: %+v", ext)
	}
}

func TestRunChainUnsetRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}, {Node: "C", Via: "D"}},
	})
	var buf bytes.Buffer
	if err := runChainUnset(&buf, path, "A"); err != nil {
		t.Fatalf("runChainUnset: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Chains) != 1 || ext.Chains[0].Node != "C" {
		t.Fatalf("wrong chain remains: %+v", ext)
	}
}

func TestRunChainSetRejectsSelfChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	err := runChainSet(&buf, path, "A", "A")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle rejection, got %v", err)
	}
}

func TestRunChainLsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Chains: []extensions.Chain{{Node: "A", Via: "B"}},
	})
	var buf bytes.Buffer
	if err := runChainLs(&buf, path, true); err != nil {
		t.Fatalf("runChainLs json: %v", err)
	}
	if !strings.Contains(buf.String(), `"node":"A"`) || !strings.Contains(buf.String(), `"via":"B"`) {
		t.Fatalf("json missing fields: %s", buf.String())
	}
}
