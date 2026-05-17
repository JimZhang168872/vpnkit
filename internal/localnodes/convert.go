package localnodes

// ToProxyMap converts a Node into a mihomo proxy map (the shape that goes
// into config.yaml's `proxies:` array). Keys "name", "type", "server",
// "port" come from the Node fields; all entries in Fields are then merged
// on top. If a parser populates Fields with one of those four reserved
// keys, the Fields value WILL OVERRIDE the struct value — keep that
// invariant by never putting reserved keys into Fields.
//
// When Node.Via is non-empty, a "dialer-proxy" entry is added so mihomo
// dials this node THROUGH the named proxy/group (multi-hop chain).
//
// Exception: when Node.Via is non-empty, the resulting `dialer-proxy` key
// is written AFTER the Fields merge — so Via overrides any Fields value
// for the same key. This is the only struct-field-wins case; treat
// `dialer-proxy` as reserved for Node.Via and never populate it via Fields.
func ToProxyMap(n Node) map[string]any {
	m := make(map[string]any, 5+len(n.Fields))
	m["name"] = n.Name
	m["type"] = n.Proto
	m["server"] = n.Server
	m["port"] = n.Port
	for k, v := range n.Fields {
		m[k] = v
	}
	if n.Via != "" {
		m["dialer-proxy"] = n.Via
	}
	return m
}
