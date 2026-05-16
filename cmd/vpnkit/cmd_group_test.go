package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"vpnkit/internal/extensions"
)

func TestRunGroupAddSelect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	opts := groupAddOpts{
		Name: "G1", Type: "select", Proxies: []string{"A", "B"},
	}
	if err := runGroupAdd(&buf, path, opts); err != nil {
		t.Fatalf("runGroupAdd: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Groups) != 1 || ext.Groups[0].Name != "G1" {
		t.Fatalf("not persisted: %+v", ext)
	}
}

func TestRunGroupAddUrlTest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	opts := groupAddOpts{
		Name: "Auto", Type: "url-test", Proxies: []string{"a", "b"},
		URL: "https://www.gstatic.com/generate_204", Interval: 300, Tolerance: 50,
	}
	if err := runGroupAdd(&buf, path, opts); err != nil {
		t.Fatalf("runGroupAdd: %v", err)
	}
	ext, _ := extensions.Load(path)
	if ext.Groups[0].URL == "" || ext.Groups[0].Interval != 300 {
		t.Fatalf("url-test fields not persisted: %+v", ext.Groups[0])
	}
}

func TestRunGroupAddRejectsBadType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	var buf bytes.Buffer
	opts := groupAddOpts{Name: "X", Type: "weird", Proxies: []string{"a"}}
	err := runGroupAdd(&buf, path, opts)
	if err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("want type error, got %v", err)
	}
}

func TestRunGroupRmRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Groups: []extensions.Group{
			{Name: "G1", Type: "select", Proxies: []string{"a"}},
			{Name: "G2", Type: "select", Proxies: []string{"b"}},
		},
	})
	var buf bytes.Buffer
	if err := runGroupRm(&buf, path, "G1"); err != nil {
		t.Fatalf("runGroupRm: %v", err)
	}
	ext, _ := extensions.Load(path)
	if len(ext.Groups) != 1 || ext.Groups[0].Name != "G2" {
		t.Fatalf("wrong remaining: %+v", ext)
	}
}

func TestRunGroupLsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.toml")
	_ = extensions.Save(path, extensions.Extensions{
		Groups: []extensions.Group{{Name: "G", Type: "select", Proxies: []string{"a"}}},
	})
	var buf bytes.Buffer
	if err := runGroupLs(&buf, path, true); err != nil {
		t.Fatalf("runGroupLs json: %v", err)
	}
	if !strings.Contains(buf.String(), `"name":"G"`) {
		t.Fatalf("json missing name: %s", buf.String())
	}
}
