package localnodes

// ToProxyMap converts a Node into a mihomo proxy map (the shape that goes
// into config.yaml's `proxies:` array). All Fields are flattened into the
// top-level map; reserved keys (name/type/server/port) come from Node.
func ToProxyMap(n Node) map[string]any {
	m := make(map[string]any, 4+len(n.Fields))
	m["name"] = n.Name
	m["type"] = n.Proto
	m["server"] = n.Server
	m["port"] = n.Port
	for k, v := range n.Fields {
		m[k] = v
	}
	return m
}
