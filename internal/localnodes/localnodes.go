// Package localnodes manages user-entered proxy nodes that supplement
// subscription-fetched ones. Persisted via store.LocalNode but the in-memory
// Manager owns all mutation paths so callers don't have to know about toml.
package localnodes

import (
	"errors"
	"sync"
)

// Node mirrors store.LocalNode but lives independently so this package has
// no dependency on store (avoids an import cycle once assembler imports
// both packages). Conversion helpers are in this package's converter.go.
type Node struct {
	Name   string
	Proto  string // ss | vmess | vless | trojan | hysteria2 | tuic
	Server string
	Port   int
	Fields map[string]any
}

// Manager is the goroutine-safe owner of a node list.
type Manager struct {
	mu    sync.Mutex
	nodes []Node
}

func New() *Manager { return &Manager{nodes: []Node{}} }

func (m *Manager) Load(initial []Node) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes = append([]Node(nil), initial...)
}

func (m *Manager) Add(n Node) error {
	if n.Name == "" {
		return errors.New("localnodes: name required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.nodes {
		if x.Name == n.Name {
			return errors.New("localnodes: duplicate name " + n.Name)
		}
	}
	m.nodes = append(m.nodes, n)
	return nil
}

func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, x := range m.nodes {
		if x.Name == name {
			m.nodes = append(m.nodes[:i], m.nodes[i+1:]...)
			return nil
		}
	}
	return errors.New("localnodes: not found " + name)
}

func (m *Manager) Get(name string) (Node, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.nodes {
		if x.Name == name {
			return x, true
		}
	}
	return Node{}, false
}

func (m *Manager) All() []Node {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Node, len(m.nodes))
	copy(out, m.nodes)
	return out
}

// Update mutates the named node atomically under the manager's lock. The
// callback runs WITH THE MUTEX HELD, so it MUST NOT call back into other
// Manager methods (Get/All/Add/Remove/Update/Load) — doing so will deadlock
// because sync.Mutex is not reentrant. Use this method only for self-contained
// mutations that read and write fields on the passed *Node.
func (m *Manager) Update(name string, mut func(*Node) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.nodes {
		if m.nodes[i].Name == name {
			return mut(&m.nodes[i])
		}
	}
	return errors.New("localnodes: not found " + name)
}
