package localnodes

// ToProxyMap converts a Node into a mihomo proxy map (the shape that goes
// into config.yaml's `proxies:` array). Keys "name", "type", "server",
// "port" come from the Node fields; all entries in Fields are then merged
// on top. If a parser populates Fields with one of those four reserved
// keys, the Fields value WILL OVERRIDE the struct value — keep that
// invariant by never putting reserved keys into Fields.
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
