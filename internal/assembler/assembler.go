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
	Mode             Mode
	GlobalTarget     string
	Subscriptions    []groups.Group
	LocalGroups      []groups.Group // one Group per enabled local-nodes-group
	LocalRules       []localrules.Rule
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	ProxyUser        string
	ProxyPass        string
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

	doc["proxies"] = emitProxies(in.Subscriptions, in.LocalGroups)
	doc["proxy-groups"] = emitProxyGroups(in.Subscriptions, in.LocalGroups, in.GlobalTarget)
	doc["rules"] = emitRules(in.Mode, in.LocalRules, in.Subscriptions)

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

func mihomoGeoxURL() map[string]string {
	const base = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	return map[string]string{
		"geoip":   base + "/geoip.metadb",
		"mmdb":    base + "/country.mmdb",
		"geosite": base + "/geosite.dat",
		"asn":     base + "/GeoLite2-ASN.mmdb",
	}
}
