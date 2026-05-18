// Package assembler builds the final mihomo config.yaml from vpnkit's
// in-memory state: subscription groups, the local-nodes group(s), local rules,
// and the top-level routing knobs (Mode + GlobalTarget).
package assembler

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/groups"
	"vpnkit/internal/localrules"
	"vpnkit/internal/rules"
)

// Mode describes the user-visible routing strategy. vpnkit always emits
// mihomo mode: rule; the effect is achieved by rewriting the rules section.
type Mode string

const (
	ModeRule   Mode = "rule"
	ModeGlobal Mode = "global"
	ModeDirect Mode = "direct"
)

// Input is the full Assemble payload. Pure value — no I/O.
type Input struct {
	Mode Mode
	// ActiveSource names the single source (subscription OR local-node
	// group) whose nodes back 🚀 Proxy AND whose rules drive routing.
	// One-at-a-time model added in rc.7 (replaces the rc.6- merge-all
	// semantics):
	//   - If ActiveSource matches an enabled subscription that carries
	//     rules → emit those rules (with target rewriting).
	//   - If ActiveSource matches a local-nodes group (which never carries
	//     rules), OR a subscription that returned no rules → fall back to
	//     the RuleTemplate (loyalsoldier / minimal / etc.).
	//   - User LocalRules are always prepended regardless.
	// Empty ActiveSource falls back to "first enabled source" by insertion
	// order so a fresh install with one sub Just Works.
	ActiveSource string
	// GlobalTarget is the *member* the top-level 🚀 Proxy Selector
	// defaults to. With the rc.7+ active-source model this is computed
	// from ActiveSource ("<active>-auto") unless the caller wants to
	// override it for tests / advanced TUI flows.
	GlobalTarget     string
	Subscriptions    []groups.Group
	LocalGroups      []groups.Group // one Group per enabled local-nodes-group
	LocalRules       []localrules.Rule
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	ProxyUser        string
	ProxyPass        string
	// RuleTemplate names an embedded baseline rule set ("loyalsoldier" /
	// "minimal" / …) whose `rule-providers` and `rules` get merged into
	// every emitted config.yaml. Empty = no template = backwards-compat
	// with rc.5- (local + subscription + MATCH only).
	RuleTemplate string
}

// Assemble produces the bytes that bootstrap atomically writes to
// ~/.config/mihomo/config.yaml. Pure function — no I/O.
func Assemble(in Input) ([]byte, error) {
	if in.MixedPort == 0 || in.ControllerPort == 0 {
		return nil, fmt.Errorf("assembler: ports must be set (got mixed=%d controller=%d)", in.MixedPort, in.ControllerPort)
	}
	if in.GlobalTarget == "" {
		in.GlobalTarget = "🚀 Proxy"
	}

	doc := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"bind-address":        "127.0.0.1",
		"mode":                "rule", // vpnkit always uses rule mode; routing knob is emulated via rules.
		"log-level":           "info",
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
		"proxies":             []any{},
		"proxy-groups":        []any{},
		"rules":               []any{},
		"geox-url":            mihomoGeoxURL(),
	}
	if in.ProxyUser != "" && in.ProxyPass != "" {
		doc["authentication"] = []string{in.ProxyUser + ":" + in.ProxyPass}
	}

	// Resolve the active source if the caller didn't specify one — first
	// enabled subscription wins, falling back to the first enabled local
	// group. Mirrors the runtime first-source nudge so a fresh install
	// with one sub never sees an empty 🚀 Proxy.
	if in.ActiveSource == "" {
		in.ActiveSource = firstEnabledSourceName(in.Subscriptions, in.LocalGroups)
	}

	doc["proxies"] = emitProxies(in.Subscriptions, in.LocalGroups)
	doc["proxy-groups"] = emitProxyGroups(in.Subscriptions, in.LocalGroups, in.ActiveSource, in.GlobalTarget)

	// Bake the rule template (rule-providers + baseline rules) into every
	// emit so vpnkit's reassembles preserve the rules mihomo downloaded
	// from the template's CDN URLs. Without this, the very first Sources
	// mutation strips rule-providers / RULE-SET rules from config.yaml
	// and the user perceives "rules vanish after first edit."
	var templateRules []any
	if in.RuleTemplate != "" {
		raw, err := rules.Load(in.RuleTemplate)
		if err != nil {
			return nil, fmt.Errorf("rule template %q: %w", in.RuleTemplate, err)
		}
		var tmpl struct {
			RuleProviders map[string]any `yaml:"rule-providers"`
			Rules         []any          `yaml:"rules"`
		}
		if err := yaml.Unmarshal(raw, &tmpl); err != nil {
			return nil, fmt.Errorf("rule template %q parse: %w", in.RuleTemplate, err)
		}
		if len(tmpl.RuleProviders) > 0 {
			doc["rule-providers"] = tmpl.RuleProviders
		}
		// Strip any trailing MATCH from the template so emitRules' own
		// `MATCH,🚀 Proxy` stays the final catch-all (one MATCH, not two).
		templateRules = tmpl.Rules
		for len(templateRules) > 0 {
			last, _ := templateRules[len(templateRules)-1].(string)
			if !strings.HasPrefix(strings.TrimSpace(last), "MATCH,") {
				break
			}
			templateRules = templateRules[:len(templateRules)-1]
		}
	}
	doc["rules"] = emitRules(in.Mode, in.LocalRules, in.Subscriptions, in.ActiveSource, templateRules)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("yaml close: %w", err)
	}

	// yaml.v3 escapes codepoints above U+FFFF (surrogate pairs) as \UXXXXXXXX.
	// Only the three emoji we actually emit (🚀 🎯 🛑) need un-escaping.
	result := strings.NewReplacer(
		`\U0001F680`, "🚀",
		`\U0001F3AF`, "🎯",
		`\U0001F6D1`, "🛑",
	).Replace(buf.String())
	return []byte(result), nil
}

// firstEnabledSourceName picks a sensible default ActiveSource when the
// caller didn't set one. Subscriptions take priority over local groups so
// "I have a paid sub" → routes through it; "I only have hand-entered
// nodes" → routes through them. Returns "" if no source is enabled.
func firstEnabledSourceName(subs []groups.Group, locals []groups.Group) string {
	for _, g := range subs {
		if g.Enabled() {
			return g.Name()
		}
	}
	for _, g := range locals {
		if g.Enabled() {
			return g.Name()
		}
	}
	return ""
}

func mihomoGeoxURL() map[string]string {
	const base = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	return map[string]string{
		"geoip":   base + "/geoip.metadb",
		"mmdb":    base + "/country.mmdb",
		"geosite": base + "/geosite.dat",
		"asn":     base + "/GeoLite2-ASN.mmdb",
	}
}
