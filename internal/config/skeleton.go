package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/rules"
)

// SkeletonInput captures the parameters needed to build an initial config.yaml.
type SkeletonInput struct {
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	LogLevel         string
	RuleTemplate     string
	ReleaseMirror    string
	// ProxyUser/ProxyPass, when both set, enable mihomo's top-level
	// `authentication` list, requiring HTTP/SOCKS proxy basic auth on mixed-port.
	ProxyUser string
	ProxyPass string
}

// BuildSkeleton assembles a complete (proxy-less) config.yaml suitable as a starting
// point before any subscription is loaded. Includes the chosen rule template and
// default proxy-groups (Proxy/Direct/Reject) so mihomo can start without errors.
func BuildSkeleton(in SkeletonInput) ([]byte, error) {
	if in.MixedPort == 0 {
		in.MixedPort = 7890
	}
	if in.ControllerPort == 0 {
		in.ControllerPort = 9090
	}
	if in.LogLevel == "" {
		in.LogLevel = "info"
	}

	template, err := rules.Load(in.RuleTemplate)
	if err != nil {
		return nil, err
	}

	base := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"bind-address":        "127.0.0.1",
		"mode":                "rule",
		"log-level":           in.LogLevel,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
		"proxies":             []any{},
		"proxy-groups": []map[string]any{
			{"name": "🚀 Proxy", "type": "select", "proxies": []string{"🎯 Direct"}},
			{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
			{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
		},
	}

	// Merge rule template (rule-providers + rules keys) over base.
	var ruleDoc map[string]any
	if err := yaml.Unmarshal(template, &ruleDoc); err != nil {
		return nil, fmt.Errorf("rule template parse: %w", err)
	}
	for k, v := range ruleDoc {
		base[k] = v
	}

	base["geox-url"] = mihomoGeoxURL(in.ReleaseMirror)

	if in.ProxyUser != "" && in.ProxyPass != "" {
		base["authentication"] = []string{in.ProxyUser + ":" + in.ProxyPass}
	}

	out, err := yaml.Marshal(base)
	if err != nil {
		return nil, err
	}
	// Unescape Unicode emoji sequences that yaml.v3 produces.
	result := strings.NewReplacer(
		`\U0001F680`, "🚀",
		`\U0001F3AF`, "🎯",
		`\U0001F6D1`, "🛑",
	).Replace(string(out))
	return []byte(result), nil
}

// mihomoGeoxURL returns the geox-url map mihomo uses to download geoip /
// geosite data at boot. When `mirror` is non-empty, GitHub URLs are prefixed
// with it (the INSTALL_MIRROR / release_mirror path). When empty, the URLs
// default to the jsdelivr CDN serving the MetaCubeX/meta-rules-dat repo's
// `@release` branch — the geo files are committed to git, so jsdelivr can
// cache them, and jsdelivr is reachable from inside the GFW without a proxy.
// This matters because mihomo refuses to start on first launch if a GEOIP /
// GEOSITE rule references a missing MMDB and the geox download times out.
func mihomoGeoxURL(mirror string) map[string]string {
	if mirror != "" {
		if mirror[len(mirror)-1] != '/' {
			mirror += "/"
		}
		base := "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
		return map[string]string{
			"geoip":   mirror + base + "/geoip.metadb",
			"mmdb":    mirror + base + "/country.mmdb",
			"geosite": mirror + base + "/geosite.dat",
			"asn":     mirror + base + "/GeoLite2-ASN.mmdb",
		}
	}
	const base = "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release"
	return map[string]string{
		"geoip":   base + "/geoip.metadb",
		"mmdb":    base + "/country.mmdb",
		"geosite": base + "/geosite.dat",
		"asn":     base + "/GeoLite2-ASN.mmdb",
	}
}
