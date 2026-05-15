// Package store reads and writes vpnkit's own config file (~/.config/vpnkit/config.toml).
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// Profile records one subscription entry.
type Profile struct {
	Name        string    `toml:"name"`
	URL         string    `toml:"url"`
	UserAgent   string    `toml:"user_agent,omitempty"`
	LastUpdated time.Time `toml:"last_updated,omitempty"`
}

// Config is vpnkit's persisted configuration.
type Config struct {
	ControllerSecret string    `toml:"controller_secret"`
	ControllerPort   int       `toml:"controller_port"`
	ReleaseMirror    string    `toml:"release_mirror"`
	ActiveProfile    string    `toml:"active_profile,omitempty"`
	RuleTemplate     string    `toml:"rule_template"`
	ServiceMode      string    `toml:"service_mode,omitempty"`
	UITheme          string    `toml:"ui_theme"`
	Profiles         []Profile `toml:"profiles"`
}

// Store wraps a Config and its on-disk location.
type Store struct {
	path string
	mu   sync.Mutex
	Cfg  Config
}

// Load reads `path`. If the file does not exist, defaults are written and returned.
func Load(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		s.Cfg = defaults()
		if err := s.Save(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, &s.Cfg); err != nil {
		return nil, err
	}
	// Apply defaults for any zero-value fields the caller relies on.
	if s.Cfg.ControllerPort == 0 {
		s.Cfg.ControllerPort = 9090
	}
	if s.Cfg.RuleTemplate == "" {
		s.Cfg.RuleTemplate = "loyalsoldier"
	}
	if s.Cfg.UITheme == "" {
		s.Cfg.UITheme = "default"
	}
	if s.Cfg.ControllerSecret == "" {
		s.Cfg.ControllerSecret = randHex(16)
	}
	return s, nil
}

// Save serializes Cfg to disk atomically (tmp + rename).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "config-*.toml.tmp")
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(tmp).Encode(s.Cfg); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func defaults() Config {
	return Config{
		ControllerSecret: randHex(16),
		ControllerPort:   9090,
		RuleTemplate:     "loyalsoldier",
		UITheme:          "default",
	}
}

func randHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
