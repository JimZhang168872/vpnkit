// Package localrules manages user-authored mihomo rule entries kept in
// store.toml under [[local_rules]]. Order matters (first match wins) and
// the Manager preserves insertion order while supporting Move for reorder.
package localrules

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
)

// Rule is one entry. Type + Payload + Target map directly to mihomo's rule
// line syntax. MATCH and FINAL have empty Payload by convention.
type Rule struct {
	Type    string
	Payload string
	Target  string
}

// Render produces the mihomo rule string. MATCH/FINAL omit the payload field.
func (r Rule) Render() string {
	if r.Type == "MATCH" || r.Type == "FINAL" {
		return r.Type + "," + r.Target
	}
	return strings.Join([]string{r.Type, r.Payload, r.Target}, ",")
}

// validTypes is the whitelist of mihomo rule types this package accepts.
// Source: https://wiki.metacubex.one/config/rules/
var validTypes = map[string]bool{
	"DOMAIN":               true,
	"DOMAIN-SUFFIX":        true,
	"DOMAIN-KEYWORD":       true,
	"DOMAIN-REGEX":         true,
	"GEOSITE":              true,
	"IP-CIDR":              true,
	"IP-CIDR6":             true,
	"IP-SUFFIX":            true,
	"IP-ASN":               true,
	"GEOIP":                true,
	"SRC-GEOIP":            true,
	"SRC-IP-ASN":           true,
	"SRC-IP-CIDR":          true,
	"SRC-IP-SUFFIX":        true,
	"DST-PORT":             true,
	"SRC-PORT":             true,
	"IN-PORT":              true,
	"IN-TYPE":              true,
	"IN-USER":              true,
	"IN-NAME":              true,
	"PROCESS-PATH":         true,
	"PROCESS-PATH-REGEX":   true,
	"PROCESS-NAME":         true,
	"PROCESS-NAME-REGEX":   true,
	"UID":                  true,
	"NETWORK":              true,
	"DSCP":                 true,
	"RULE-SET":             true,
	"AND":                  true,
	"OR":                   true,
	"NOT":                  true,
	"SUB-RULE":             true,
	"MATCH":                true,
	"FINAL":                true,
}

// Validate returns nil if the Rule is acceptable for assembly.
func Validate(r Rule) error {
	if !validTypes[r.Type] {
		return fmt.Errorf("localrules: unknown rule type %q", r.Type)
	}
	if r.Type != "MATCH" && r.Type != "FINAL" && r.Payload == "" {
		return fmt.Errorf("localrules: type %q requires payload", r.Type)
	}
	if r.Target == "" {
		return errors.New("localrules: target required")
	}
	// Reject embedded control characters in payload/target — they break
	// the emitted YAML and lead to opaque mihomo reload failures. Each
	// rule is a single line, so any raw \n / \r / \t in payload would
	// split the rule mid-emission and the rest of the file would be
	// rejected as a parse error.
	for _, s := range []string{r.Payload, r.Target} {
		for _, c := range s {
			if (c < 0x20 && c != 0) || c == 0x7f {
				return fmt.Errorf("localrules: control character (0x%02x) in payload/target", c)
			}
		}
	}
	// Per-type payload validation — catches garbage at add time rather
	// than after mihomo refuses to reload the whole config later.
	switch r.Type {
	case "IP-CIDR", "IP-CIDR6", "SRC-IP-CIDR":
		if _, _, err := net.ParseCIDR(r.Payload); err != nil {
			return fmt.Errorf("localrules: %s payload %q is not a valid CIDR: %v", r.Type, r.Payload, err)
		}
	case "DOMAIN-REGEX":
		if _, err := regexp.Compile(r.Payload); err != nil {
			return fmt.Errorf("localrules: DOMAIN-REGEX payload %q is not a valid regexp: %v", r.Payload, err)
		}
	case "DST-PORT", "SRC-PORT":
		if err := validatePortPayload(r.Payload); err != nil {
			return fmt.Errorf("localrules: %s: %v", r.Type, err)
		}
	}
	return nil
}

// validatePortPayload accepts "80" or "80-443"; rejects non-numeric and
// out-of-range values.
func validatePortPayload(p string) error {
	parts := strings.SplitN(p, "-", 2)
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("empty port component in %q", p)
		}
		n := 0
		for _, c := range part {
			if c < '0' || c > '9' {
				return fmt.Errorf("port %q not numeric", part)
			}
			n = n*10 + int(c-'0')
			if n > 65535 {
				return fmt.Errorf("port %s out of range (1-65535)", part)
			}
		}
		if n < 1 {
			return fmt.Errorf("port %s out of range (1-65535)", part)
		}
	}
	return nil
}

// Manager owns the in-memory rules list; persistence is done by callers
// translating to []store.LocalRule.
type Manager struct {
	mu    sync.Mutex
	rules []Rule
}

// New returns an empty Manager ready to use.
func New() *Manager { return &Manager{rules: []Rule{}} }

// Load replaces the current rule list with a copy of initial.
func (m *Manager) Load(initial []Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append([]Rule(nil), initial...)
}

// Add validates r and appends it to the list.
func (m *Manager) Add(r Rule) error {
	if err := Validate(r); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, r)
	return nil
}

// Remove deletes the rule at idx. Returns error if idx is out of range.
func (m *Manager) Remove(idx int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx < 0 || idx >= len(m.rules) {
		return fmt.Errorf("localrules: index %d out of range", idx)
	}
	m.rules = append(m.rules[:idx], m.rules[idx+1:]...)
	return nil
}

// Move relocates the rule at from to position to. Both indices are checked
// against the current length before any mutation. The element is extracted
// first (shifting remaining elements left by one), then re-inserted at to
// (which may now address the shifted slice).
func (m *Manager) Move(from, to int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if from < 0 || from >= len(m.rules) || to < 0 || to >= len(m.rules) {
		return fmt.Errorf("localrules: bad indices %d→%d", from, to)
	}
	if from == to {
		return nil
	}
	r := m.rules[from]
	m.rules = append(m.rules[:from], m.rules[from+1:]...)
	m.rules = append(m.rules[:to], append([]Rule{r}, m.rules[to:]...)...)
	return nil
}

// All returns a copy of the current rule list.
func (m *Manager) All() []Rule {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Rule, len(m.rules))
	copy(out, m.rules)
	return out
}
