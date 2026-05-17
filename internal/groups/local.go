package groups

import (
	"vpnkit/internal/localrules"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/subscription/proto"
)

type localNodesGroup struct {
	name string
	mgr  *localnodes.Manager
}

// NewLocalNodesGroup wraps a localnodes.Manager. Always Enabled; the user's
// LocalRules subsystem provides routing, so this Group has no own Rules.
func NewLocalNodesGroup(name string, m *localnodes.Manager) Group {
	return &localNodesGroup{name: name, mgr: m}
}

func (g *localNodesGroup) Name() string  { return g.name }
func (g *localNodesGroup) Kind() Kind    { return KindLocalNodes }
func (g *localNodesGroup) Enabled() bool { return true }

func (g *localNodesGroup) Proxies() []proto.Proxy {
	all := g.mgr.All()
	out := make([]proto.Proxy, len(all))
	for i, n := range all {
		out[i] = proto.Proxy(localnodes.ToProxyMap(n))
	}
	return out
}

func (g *localNodesGroup) Rules() []localrules.Rule { return nil }
