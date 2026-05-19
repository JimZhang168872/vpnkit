package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	"vpnkit/internal/config"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

// Subscription Result persistence across CLI invocations.
//
// Pre-rc.7, Pipeline.subResults was an in-memory map populated only when a
// subscription was fetched in the current process. Any other CLI mutation
// path (`vpnkit subs add`, `local-nodes add`, `active`, etc.) ran with an
// empty map, so Assemble emitted a config.yaml MISSING the sub's proxies
// and proxy-groups — but topProxyMembersFor still referenced "<sub>-auto",
// producing an invalid mihomo config every mutation. This file gives
// Pipeline a disk-backed cache so non-fetch mutations still see node data.
//
// Format: one JSON file per subscription at <cacheDir>/sub-cache/<safe>.json.
// We persist .Source / .Proxies / .Raw; .Errors are transient diagnostics
// that don't survive serialization meaningfully.

type subResultDisk struct {
	Source  string         `json:"source"`
	Proxies []proto.Proxy  `json:"proxies"`
	Raw     map[string]any `json:"raw,omitempty"`
}

// subCacheDir returns the directory holding cached subscription results.
// Created lazily by saveSubResult.
func subCacheDir(base string) string {
	return filepath.Join(base, "sub-cache")
}

// subCachePath returns the file path for one subscription's cached result.
// Names get URL-escaped so user-chosen names containing "/", spaces, or
// other reserved characters can't escape the cache dir.
func subCachePath(base, name string) string {
	return filepath.Join(subCacheDir(base), url.PathEscape(name)+".json")
}

// saveSubResult writes res to <cacheDir>/sub-cache/<name>.json atomically.
// If cacheDir is empty the call is a no-op (tests / contexts without a
// resolved XDG cache).
func saveSubResult(cacheDir, name string, res *subscription.Result) error {
	if cacheDir == "" || res == nil {
		return nil
	}
	if err := os.MkdirAll(subCacheDir(cacheDir), 0o700); err != nil {
		return fmt.Errorf("sub-cache mkdir: %w", err)
	}
	d := subResultDisk{Source: res.Source, Proxies: res.Proxies, Raw: res.Raw}
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("sub-cache marshal: %w", err)
	}
	return config.AtomicWrite(subCachePath(cacheDir, name), data, 0o600)
}

// loadSubResult reads a previously saved Result. Returns (nil, nil) when
// the cache file doesn't exist — that's not an error, it's "haven't fetched
// yet in this install."
func loadSubResult(cacheDir, name string) (*subscription.Result, error) {
	if cacheDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(subCachePath(cacheDir, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("sub-cache read %s: %w", name, err)
	}
	var d subResultDisk
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("sub-cache parse %s: %w", name, err)
	}
	return &subscription.Result{Source: d.Source, Proxies: d.Proxies, Raw: d.Raw}, nil
}

// dropSubResult removes the cache file for a removed/renamed subscription.
// Non-existent file is not an error.
func dropSubResult(cacheDir, name string) error {
	if cacheDir == "" {
		return nil
	}
	err := os.Remove(subCachePath(cacheDir, name))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("sub-cache drop %s: %w", name, err)
	}
	return nil
}
