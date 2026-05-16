// Package extensions models vpnkit's user-controlled overlay on top of the
// subscription-generated mihomo config: per-node dialer-proxy chains and
// custom proxy-groups. Stored in ~/.config/vpnkit/extensions.toml.
package extensions

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Chain pins one subscription node to dial through an upstream node/group
// (mihomo `dialer-proxy` field). Both Node and Via are mihomo proxy names.
type Chain struct {
	Node string `toml:"node"`
	Via  string `toml:"via"`
}

// Group is a user-defined proxy-group appended to the assembled config after
// any subscription-supplied or synthesized groups.
type Group struct {
	Name      string   `toml:"name"`
	Type      string   `toml:"type"` // select | url-test | fallback | load-balance | relay
	Proxies   []string `toml:"proxies"`
	URL       string   `toml:"url,omitempty"`
	Interval  int      `toml:"interval,omitempty"`
	Tolerance int      `toml:"tolerance,omitempty"`
}

// Extensions is the full content of extensions.toml.
type Extensions struct {
	SchemaVersion int     `toml:"schema_version"`
	Chains        []Chain `toml:"chains"`
	Groups        []Group `toml:"groups"`
}

// Load reads `path`. A missing file is treated as an empty Extensions value
// (not an error) so callers can run before the user has created one.
func Load(path string) (Extensions, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Extensions{}, nil
	}
	if err != nil {
		return Extensions{}, err
	}
	var ext Extensions
	if err := toml.Unmarshal(data, &ext); err != nil {
		return Extensions{}, err
	}
	return ext, nil
}

// Save writes `ext` to `path` atomically (tmp + rename) with mode 0600.
func Save(path string, ext Extensions) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if ext.SchemaVersion == 0 {
		ext.SchemaVersion = 1
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "extensions-*.toml.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Chmod(0o600)
	defer os.Remove(tmpName)
	if err := toml.NewEncoder(tmp).Encode(ext); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
