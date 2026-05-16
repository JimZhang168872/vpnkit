// Package profiles is the high-level facade combining subscription fetch + convert +
// assemble + config write, plus storing profile metadata in memory.
package profiles

import (
	"context"
	"errors"
	"sync"
	"time"

	"vpnkit/internal/config"
	"vpnkit/internal/subscription"
)

// Profile is one subscription entry tracked in-memory by the manager.
type Profile struct {
	Name        string
	URL         string
	UserAgent   string
	LastUpdated time.Time
	NodeCount   int
}

// Config configures Manager.
type Config struct {
	ConfigYAMLPath   string
	PatchPath        string
	ControllerPort   int
	ControllerSecret string
	MixedPort        int
	RuleTemplate     string
	ReleaseMirror    string
	ProxyUser        string
	ProxyPass        string
}

// Manager holds the active profile list and writes config files.
type Manager struct {
	cfg      Config
	mu       sync.Mutex
	profiles []Profile
	active   string
	onChange func()
}

// New constructs a Manager.
func New(cfg Config) *Manager { return &Manager{cfg: cfg} }

// SetOnChange installs a callback fired after each profile mutation.
func (m *Manager) SetOnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

func (m *Manager) fireChange() {
	m.mu.Lock()
	cb := m.onChange
	m.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// Load replaces the profile list (e.g. from store.Cfg.Profiles).
func (m *Manager) Load(list []Profile, active string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles = list
	m.active = active
}

// List returns profile names in insertion order.
func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, len(m.profiles))
	for i, p := range m.profiles {
		names[i] = p.Name
	}
	return names
}

// All returns a copy of all profile entries.
func (m *Manager) All() []Profile {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Profile, len(m.profiles))
	copy(out, m.profiles)
	return out
}

// Active returns the currently-active profile name.
func (m *Manager) Active() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// SetActive marks a profile name as active.
func (m *Manager) SetActive(name string) {
	m.mu.Lock()
	m.active = name
	m.mu.Unlock()
	m.fireChange()
}

// Add registers a new profile.
func (m *Manager) Add(p Profile) error {
	m.mu.Lock()
	for _, e := range m.profiles {
		if e.Name == p.Name {
			m.mu.Unlock()
			return errors.New("profiles: duplicate name")
		}
	}
	m.profiles = append(m.profiles, p)
	if m.active == "" {
		m.active = p.Name
	}
	m.mu.Unlock()
	m.fireChange()
	return nil
}

// Remove deletes a profile by name.
func (m *Manager) Remove(name string) {
	m.mu.Lock()
	out := m.profiles[:0]
	for _, p := range m.profiles {
		if p.Name != name {
			out = append(out, p)
		}
	}
	m.profiles = out
	if m.active == name {
		m.active = ""
	}
	m.mu.Unlock()
	m.fireChange()
}

// Update fetches the named profile's URL, converts, assembles, and writes config.yaml.
func (m *Manager) Update(ctx context.Context, name string) (int, error) {
	m.mu.Lock()
	var p *Profile
	for i := range m.profiles {
		if m.profiles[i].Name == name {
			p = &m.profiles[i]
			break
		}
	}
	cfg := m.cfg
	m.mu.Unlock()
	if p == nil {
		return 0, errors.New("profiles: not found")
	}

	body, err := subscription.Fetch(ctx, p.URL, p.UserAgent)
	if err != nil {
		return 0, err
	}
	res, err := subscription.Convert(body)
	if err != nil {
		return 0, err
	}
	yamlBytes, err := subscription.Assemble(subscription.AssembleInput{
		Result:           res,
		MixedPort:        cfg.MixedPort,
		ControllerPort:   cfg.ControllerPort,
		ControllerSecret: cfg.ControllerSecret,
		RuleTemplate:     cfg.RuleTemplate,
		PatchPath:        cfg.PatchPath,
		ReleaseMirror:    cfg.ReleaseMirror,
		ProxyUser:        cfg.ProxyUser,
		ProxyPass:        cfg.ProxyPass,
	})
	if err != nil {
		return 0, err
	}
	if err := config.AtomicWrite(cfg.ConfigYAMLPath, yamlBytes, 0o600); err != nil {
		return 0, err
	}

	m.mu.Lock()
	p.LastUpdated = time.Now()
	p.NodeCount = len(res.Proxies)
	m.mu.Unlock()
	m.fireChange()
	return len(res.Proxies), nil
}
