package subscription

import "vpnkit/internal/subscription/proto"

// SynthesizeGroups builds the default 4-group set (Proxy / Auto / Direct / Reject)
// for subscriptions that don't supply their own proxy-groups.
func SynthesizeGroups(proxies []proto.Proxy) []map[string]any {
	names := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if n, ok := p["name"].(string); ok && n != "" {
			names = append(names, n)
		}
	}
	proxyGroup := append([]string{"♻️ Auto", "🎯 Direct"}, names...)
	return []map[string]any{
		{"name": "🚀 Proxy", "type": "select", "proxies": proxyGroup},
		{"name": "♻️ Auto", "type": "url-test", "proxies": names,
			"url": "https://www.gstatic.com/generate_204", "interval": 300, "tolerance": 50},
		{"name": "🎯 Direct", "type": "select", "proxies": []string{"DIRECT"}},
		{"name": "🛑 Reject", "type": "select", "proxies": []string{"REJECT", "DIRECT"}},
	}
}
