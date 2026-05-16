package subscription

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"vpnkit/internal/extensions"
	"vpnkit/internal/rules"
	"vpnkit/internal/subscription/proto"
)

// AssembleInput drives a single Assemble call.
type AssembleInput struct {
	Result           Result
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	LogLevel         string
	RuleTemplate     string
	Extensions       extensions.Extensions
	ProxyUser        string
	ProxyPass        string
}

// Assemble produces the final config.yaml bytes by combining:
// base skeleton + subscription proxies + groups (synthesized or from clash) +
// rules + extensions overlay (chains + custom groups).
func Assemble(in AssembleInput) ([]byte, error) {
	if in.MixedPort == 0 {
		in.MixedPort = 7890
	}
	if in.ControllerPort == 0 {
		in.ControllerPort = 9090
	}
	if in.LogLevel == "" {
		in.LogLevel = "info"
	}
	ruleYAML, err := rules.Load(in.RuleTemplate)
	if err != nil {
		return nil, err
	}
	var ruleDoc map[string]any
	if err := yaml.Unmarshal(ruleYAML, &ruleDoc); err != nil {
		return nil, fmt.Errorf("rule template parse: %w", err)
	}

	doc := map[string]any{
		"mixed-port":          in.MixedPort,
		"allow-lan":           false,
		"bind-address":        "127.0.0.1",
		"mode":                "rule",
		"log-level":           in.LogLevel,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", in.ControllerPort),
		"secret":              in.ControllerSecret,
	}
	if in.ProxyUser != "" && in.ProxyPass != "" {
		doc["authentication"] = []string{in.ProxyUser + ":" + in.ProxyPass}
	}

	rawProxies := make([]any, 0, len(in.Result.Proxies))
	for _, p := range in.Result.Proxies {
		rawProxies = append(rawProxies, map[string]any(p))
	}
	doc["proxies"] = rawProxies

	if in.Result.Source == "clash" && in.Result.Raw != nil {
		if g, ok := in.Result.Raw["proxy-groups"]; ok {
			doc["proxy-groups"] = g
		}
	}
	if _, has := doc["proxy-groups"]; !has {
		doc["proxy-groups"] = groupsToAny(SynthesizeGroups(in.Result.Proxies))
	}

	for k, v := range ruleDoc {
		doc[k] = v
	}

	doc["geox-url"] = mihomoGeoxURL()

	if err := extensions.Apply(doc, in.Extensions); err != nil {
		return nil, fmt.Errorf("extensions: %w", err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	_ = enc.Close()

	// Unescape Unicode emoji sequences that yaml.v3 produces.
	result := strings.NewReplacer(
		`\U0001F680`, "🚀",
		`\U0001F3AF`, "🎯",
		`\U0001F6D1`, "🛑",
		`\U000267B`, "♻️",
	).Replace(string(buf.Bytes()))
	return []byte(result), nil
}

func groupsToAny(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, g := range in {
		out[i] = g
	}
	return out
}

// mihomoGeoxURL returns the geox-url map for mihomo, pointing at
// MetaCubeX/meta-rules-dat GitHub Releases directly. No mirror layer —
// users behind restrictive networks should configure HTTPS_PROXY before
// running so SmartClient routes through their existing proxy.
func mihomoGeoxURL() map[string]string {
	const base = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest"
	return map[string]string{
		"geoip":   base + "/geoip.metadb",
		"mmdb":    base + "/country.mmdb",
		"geosite": base + "/geosite.dat",
		"asn":     base + "/GeoLite2-ASN.mmdb",
	}
}

var _ proto.Proxy
