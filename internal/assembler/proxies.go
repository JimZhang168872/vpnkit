package assembler

import (
	"fmt"

	"vpnkit/internal/groups"
	"vpnkit/internal/subscription/proto"
)

// emitProxies returns a flat slice of mihomo proxy maps with every node's
// name rewritten to "<group>:<original-name>" so cross-group duplicates
// don't collide in mihomo's flat namespace. localGroups is one Group per
// enabled local-nodes-group.
func emitProxies(subs []groups.Group, localGroups []groups.Group) []any {
	out := []any{}
	for _, g := range subs {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			out = append(out, namespaced(g.Name(), p))
		}
	}
	for _, g := range localGroups {
		if !g.Enabled() {
			continue
		}
		for _, p := range g.Proxies() {
			out = append(out, namespaced(g.Name(), p))
		}
	}
	return out
}

func namespaced(groupName string, p proto.Proxy) map[string]any {
	dup := make(map[string]any, len(p))
	for k, v := range p {
		dup[k] = v
	}
	origName, _ := dup["name"].(string)
	dup["name"] = fmt.Sprintf("%s:%s", groupName, origName)
	return dup
}
