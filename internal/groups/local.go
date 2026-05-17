package groups

import (
	"vpnkit/internal/localrules"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription/proto"
)

type localNodesGroup struct {
	name          string
	mgr           *localnodes.Manager
	filterByGroup bool // true → only nodes whose .Group == name; false → all
}

// NewLocalNodesGroup (unchanged signature) keeps emitting every node.
func NewLocalNodesGroup(name string, m *localnodes.Manager) Group {
	return &localNodesGroup{name: name, mgr: m, filterByGroup: false}
}

// NewLocalNodesGroupForGroup wraps a localnodes.Manager but exposes only
// the subset of nodes whose Group field matches groupName. Used by the
// assembler to emit one mihomo proxy-group per user-defined local group
// (e.g. "home", "office") instead of a single hardcoded "local" group.
func NewLocalNodesGroupForGroup(groupName string, m *localnodes.Manager) Group {
	return &localNodesGroup{name: groupName, mgr: m, filterByGroup: true}
}

func (g *localNodesGroup) Name() string  { return g.name }
func (g *localNodesGroup) Kind() Kind    { return KindLocalNodes }
func (g *localNodesGroup) Enabled() bool { return true }

func (g *localNodesGroup) Proxies() []proto.Proxy {
	all := g.mgr.All()
	out := make([]proto.Proxy, 0, len(all))
	for _, n := range all {
		if g.filterByGroup && n.Group != g.name {
			continue
		}
		out = append(out, proto.Proxy(localnodes.ToProxyMap(n)))
	}
	return out
}

func (g *localNodesGroup) Rules() []localrules.Rule { return nil }
