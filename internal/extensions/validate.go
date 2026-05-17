package extensions

import (
	"errors"
	"fmt"
)

// supportedGroupTypes lists the mihomo proxy-group types vpnkit's CLI/TUI
// can safely round-trip. mihomo supports more (e.g. select with `lazy`) but
// the Group struct is intentionally a small subset.
var supportedGroupTypes = map[string]bool{
	"select":       true,
	"url-test":     true,
	"fallback":     true,
	"load-balance": true,
	"relay":        true,
}

// Validate performs shape-only checks: no missing required fields, no
// duplicate chain origins or group names, no chain cycles, group type
// within the supported whitelist. It does NOT cross-check Chain.Node /
// Chain.Via / Group.Proxies against the running mihomo's proxy list —
// that happens at Apply time as a warn-only path.
func Validate(ext Extensions) error {
	if err := validateChains(ext.Chains); err != nil {
		return err
	}
	if err := validateGroups(ext.Groups); err != nil {
		return err
	}
	return nil
}

func validateChains(chains []Chain) error {
	seen := map[string]bool{}
	graph := map[string]string{}
	for i, c := range chains {
		if c.Node == "" {
			return fmt.Errorf("chains[%d]: chain.node empty", i)
		}
		if c.Via == "" {
			return fmt.Errorf("chains[%d]: chain.via empty", i)
		}
		if seen[c.Node] {
			return fmt.Errorf("chains[%d]: duplicate node %q", i, c.Node)
		}
		seen[c.Node] = true
		graph[c.Node] = c.Via
	}
	// DFS each node; if we revisit a node already on the current stack, cycle.
	for start := range graph {
		stack := map[string]bool{}
		cur := start
		for {
			if stack[cur] {
				return fmt.Errorf("chain cycle detected at node %q", cur)
			}
			stack[cur] = true
			next, ok := graph[cur]
			if !ok {
				break
			}
			cur = next
		}
	}
	return nil
}

func validateGroups(groups []Group) error {
	seen := map[string]bool{}
	for i, g := range groups {
		if g.Name == "" {
			return fmt.Errorf("groups[%d]: name empty", i)
		}
		if seen[g.Name] {
			return fmt.Errorf("groups[%d]: duplicate name %q", i, g.Name)
		}
		seen[g.Name] = true
		if !supportedGroupTypes[g.Type] {
			return fmt.Errorf("groups[%d]: unsupported type %q (want one of select|url-test|fallback|load-balance|relay)", i, g.Type)
		}
		if len(g.Proxies) == 0 {
			return fmt.Errorf("groups[%d] %q: proxies must be non-empty", i, g.Name)
		}
	}
	return nil
}

// ErrCycle is returned when chain validation finds a cycle. Exposed so
// CLI/TUI callers can match without parsing the message.
var ErrCycle = errors.New("chain cycle")
