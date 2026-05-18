// Package sources holds shared validation for the subscription and
// local-node-group namespace. Both CLI dispatchers and the TUI's
// Pipeline mutators MUST funnel name validation through this package so
// the rules are enforced uniformly regardless of entry point.
//
// Pre-rc.7 the validator lived in `cmd/vpnkit/cmd_subs.go` and only the
// CLI called it; TUI paths through `Pipeline.AddSubscription` /
// `AddLocalGroup` / `RenameLocalGroup` silently accepted names like
// `bad:name` (contains the `<src>:<node>` separator), `DIRECT` (mihomo
// reserved), or shell-metacharacter strings.
package sources

import (
	"fmt"
	"strings"
)

// MaxNameLen caps subscription / local-group names. Pre-rc.7 a 1000-char
// name silently round-tripped into the assembled YAML and produced
// pathological proxy-group entries.
const MaxNameLen = 64

// reservedNames is the case-insensitive blocklist of mihomo built-in
// policy targets. Allowing them as source names would emit a YAML with
// a custom group that collides with mihomo's own resolution rules.
var reservedNames = map[string]bool{
	"DIRECT":      true,
	"REJECT":      true,
	"REJECT-DROP": true,
	"PASS":        true,
	"COMPATIBLE":  true,
	"GLOBAL":      true,
	"🚀 PROXY":   true,
	"🎯 DIRECT":  true,
	"🛑 REJECT":  true,
}

// shellMetaChars are unsafe in source names because downstream scripts
// frequently interpolate `vpnkit subs list` / `active` output into shell
// commands. `$(whoami)` etc. would execute on those consumers.
const shellMetaChars = "$`;|&<>(){}[]\\\"'*?!#~,"

// ValidateName rejects names that would corrupt the routing namespace or
// be unsafe downstream:
//   - empty / whitespace-only
//   - control characters (\x00..\x1f, \x7f) — newline silently truncates
//     or breaks the emitted YAML
//   - shell metacharacters → script-interpolation hazard
//   - leading `-` → looks like a CLI flag to downstream parsers
//   - `/` or `:` → used as path / `<src>:<node>` separator syntax
//   - whitespace anywhere → tabs/newlines mid-name corrupt YAML emission
//   - longer than MaxNameLen
//   - matching a mihomo built-in (case-insensitive)
//
// Subscriptions and local-node groups share this namespace (both emit
// `<name>` and `<name>-auto` proxy-groups). Cross-source name collisions
// are checked separately by callers — this function only validates the
// shape of a single name.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name cannot be empty or whitespace")
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("name too long (%d > %d chars)", len(name), MaxNameLen)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("name %q starts with `-` — looks like a CLI flag, pick a different prefix", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("name contains control character (0x%02x)", r)
		}
		if r == ' ' || r == '\t' {
			return fmt.Errorf("name %q contains whitespace — replace with `-` or `_`", name)
		}
		if r == '/' || r == ':' {
			return fmt.Errorf("name contains reserved character %q (used in <src>:<node> / path syntax)", r)
		}
		if strings.ContainsRune(shellMetaChars, r) {
			return fmt.Errorf("name contains shell metacharacter %q — unsafe for scripts that interpolate the name", r)
		}
	}
	if reservedNames[strings.ToUpper(name)] {
		return fmt.Errorf("name %q is reserved by mihomo (case-insensitive) — pick a different one", name)
	}
	return nil
}

// ValidateNodeName is a looser sibling of ValidateName for hand-entered
// local-node names. Subscription feeds frequently use emoji, spaces and
// parentheses ("🇯🇵 日本JP1", "美国直连(IEPL)", etc.) — accepting those
// verbatim from feeds means CLI-added node names should ALSO allow them
// for visual consistency. But shell metacharacters and control chars
// are still rejected so a user-entered `$(whoami)` doesn't round-trip
// into scripts.
func ValidateNodeName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("name too long (%d > %d chars)", len(name), MaxNameLen)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("name %q starts with `-` — looks like a CLI flag", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("name contains control character (0x%02x)", r)
		}
		if strings.ContainsRune(shellMetaChars, r) {
			return fmt.Errorf("name contains shell metacharacter %q — unsafe for scripts that interpolate the name", r)
		}
	}
	return nil
}
