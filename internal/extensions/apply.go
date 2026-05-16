package extensions

// Apply mutates `doc` (an unmarshalled mihomo config) in place to reflect
// `ext`. Specifically:
//   - For each Chain, find the proxy whose name == chain.Node in doc["proxies"]
//     and set its "dialer-proxy" key to chain.Via. Missing nodes are skipped
//     silently (subscription may not include the node anymore).
//   - Each Group is appended to doc["proxy-groups"], preserving order.
//     If doc has no "proxy-groups" key it is initialized to an empty slice
//     before appending.
//
// Returns nil unless `doc` is structurally malformed. Live cross-checking
// against known proxies happens earlier in Validate.
func Apply(doc map[string]any, ext Extensions) error {
	if len(ext.Chains) > 0 {
		proxies, _ := doc["proxies"].([]any)
		index := map[string]map[string]any{}
		for _, p := range proxies {
			m, ok := p.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			if name != "" {
				index[name] = m
			}
		}
		for _, c := range ext.Chains {
			if m, ok := index[c.Node]; ok {
				m["dialer-proxy"] = c.Via
			}
		}
	}
	if len(ext.Groups) > 0 {
		groups, _ := doc["proxy-groups"].([]any)
		for _, g := range ext.Groups {
			groups = append(groups, groupToMap(g))
		}
		doc["proxy-groups"] = groups
	}
	return nil
}

// groupToMap converts a Group to mihomo's untyped map shape so it round-trips
// through yaml.v3 without surprises.
func groupToMap(g Group) map[string]any {
	out := map[string]any{
		"name":    g.Name,
		"type":    g.Type,
		"proxies": stringsToAny(g.Proxies),
	}
	if g.URL != "" {
		out["url"] = g.URL
	}
	if g.Interval != 0 {
		out["interval"] = g.Interval
	}
	if g.Tolerance != 0 {
		out["tolerance"] = g.Tolerance
	}
	return out
}

func stringsToAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
