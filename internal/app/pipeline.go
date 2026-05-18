package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"vpnkit/internal/assembler"
	"vpnkit/internal/config"
	"vpnkit/internal/groups"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/localrules"
	"vpnkit/internal/store"
	"vpnkit/internal/subscription"
)

// Pipeline is the v1.0.0 replacement for profiles.Manager. It owns the
// in-memory state for all subscription Groups + the LocalNodes Group +
// LocalRules + cached fetch results, and produces a config.yaml on Apply.
type Pipeline struct {
	store          *store.Store
	configYAMLPath string

	mu         sync.Mutex
	localNodes *localnodes.Manager
	localRules *localrules.Manager
	// subResults caches one fetched+converted subscription Result per name.
	// Entries for removed subscriptions are NOT auto-purged in this phase —
	// Phase 7's `subs rm` CLI must call DropSubscription (TODO) or the next
	// Assemble will silently include stale data via store re-lookup. Today
	// Assemble skips groups whose name isn't in store.Cfg.Subscriptions, so
	// the leak is contained; Phase 7 should still purge for memory hygiene.
	subResults map[string]*subscription.Result
}

// NewPipeline constructs a Pipeline and loads existing local state from the store.
func NewPipeline(st *store.Store, configYAMLPath string) *Pipeline {
	pl := &Pipeline{
		store:          st,
		configYAMLPath: configYAMLPath,
		localNodes:     localnodes.New(),
		localRules:     localrules.New(),
		subResults:     map[string]*subscription.Result{},
	}
	pl.localNodes.Load(toLocalNodes(st.Cfg.LocalNodes))
	pl.localRules.Load(toLocalRules(st.Cfg.LocalRules))
	return pl
}

func toLocalNodes(in []store.LocalNode) []localnodes.Node {
	out := make([]localnodes.Node, len(in))
	for i, x := range in {
		out[i] = localnodes.Node{
			Name:   x.Name,
			Group:  x.Group,
			Via:    x.Via,
			Proto:  x.Proto,
			Server: x.Server,
			Port:   x.Port,
			Fields: x.Fields,
		}
	}
	return out
}

func toLocalRules(in []store.LocalRule) []localrules.Rule {
	out := make([]localrules.Rule, len(in))
	for i, x := range in {
		out[i] = localrules.Rule{Type: x.Type, Payload: x.Payload, Target: x.Target}
	}
	return out
}

// RefreshSubscription fetches one named subscription, parses it, and caches
// the result. Returns the node count. Re-looks up the subscription by name
// after the fetch so the in-memory store.Subscription slice can be safely
// mutated by callers (e.g. CLI subs add/rm) while a refresh is in flight.
//
// Concurrent calls for the same name produce a duplicate fetch and a
// last-write-wins cached result; we accept that cost rather than serialize
// network I/O across all refreshes.
func (p *Pipeline) RefreshSubscription(ctx context.Context, name string) (int, error) {
	p.mu.Lock()
	var url, ua string
	found := false
	for _, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			url = s.URL
			ua = s.UserAgent
			found = true
			break
		}
	}
	p.mu.Unlock()
	if !found {
		return 0, fmt.Errorf("subscription %q not found", name)
	}

	body, err := subscription.Fetch(ctx, url, ua)
	if err != nil {
		return 0, err
	}
	res, err := subscription.Convert(body)
	if err != nil {
		return 0, err
	}

	p.mu.Lock()
	// Re-look up by name — the subscription may have been removed or
	// renamed while we were fetching.
	idx := -1
	for i, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		p.mu.Unlock()
		return 0, fmt.Errorf("subscription %q removed during refresh", name)
	}
	p.subResults[name] = &res
	p.store.Cfg.Subscriptions[idx].LastUpdated = time.Now()
	p.store.Cfg.Subscriptions[idx].NodeCount = len(res.Proxies)
	p.mu.Unlock()

	if err := p.store.Save(); err != nil {
		return 0, fmt.Errorf("save store: %w", err)
	}
	return len(res.Proxies), nil
}

// Assemble produces the config.yaml for the current state and writes it.
func (p *Pipeline) Assemble() error {
	p.mu.Lock()
	subs := make([]groups.Group, 0, len(p.store.Cfg.Subscriptions))
	for _, s := range p.store.Cfg.Subscriptions {
		if !s.Enabled {
			continue
		}
		res := p.subResults[s.Name]
		if res == nil {
			// Fetch has not happened this run; skip — status TUI will surface stale.
			continue
		}
		subs = append(subs, groups.NewSubscriptionGroup(s.Name, true, res))
	}
	// Build one Group per enabled local-nodes-group.
	var localGroups []groups.Group
	for _, g := range p.store.Cfg.LocalNodeGroups {
		if !g.Enabled {
			continue
		}
		localGroups = append(localGroups, groups.NewLocalNodesGroupForGroup(g.Name, p.localNodes))
	}
	cfg := p.store.Cfg
	p.mu.Unlock()

	bytes_, err := assembler.Assemble(assembler.Input{
		Mode:             assembler.Mode(cfg.Mode),
		GlobalTarget:     cfg.GlobalTarget,
		Subscriptions:    subs,
		LocalGroups:      localGroups,
		LocalRules:       p.localRules.All(),
		MixedPort:        cfg.MixedPort,
		ControllerPort:   cfg.ControllerPort,
		ControllerSecret: cfg.ControllerSecret,
		ProxyUser:        cfg.ProxyUser,
		ProxyPass:        cfg.ProxyPass,
	})
	if err != nil {
		return err
	}
	if err := config.AtomicWrite(p.configYAMLPath, bytes_, 0o600); err != nil {
		return err
	}
	return nil
}

// LocalNodes returns the local nodes manager for the TUI tabs.
func (p *Pipeline) LocalNodes() *localnodes.Manager { return p.localNodes }

// LocalRules returns the local rules manager for the TUI tabs.
func (p *Pipeline) LocalRules() *localrules.Manager { return p.localRules }

// SubscriptionNodes returns the cached proxy list for a named subscription as
// a slice of (name, proto, server, port) tuples. Returns nil when no fetch
// result exists yet (subscription not yet refreshed this session).
// Safe for concurrent reads.
func (p *Pipeline) SubscriptionNodes(name string) []SubNode {
	p.mu.Lock()
	defer p.mu.Unlock()
	res := p.subResults[name]
	if res == nil {
		return nil
	}
	out := make([]SubNode, 0, len(res.Proxies))
	for _, proxy := range res.Proxies {
		var n SubNode
		if v, ok := proxy["name"].(string); ok {
			n.Name = v
		}
		if v, ok := proxy["type"].(string); ok {
			n.Proto = v
		}
		if v, ok := proxy["server"].(string); ok {
			n.Server = v
		}
		switch pt := proxy["port"].(type) {
		case int:
			n.Port = pt
		case float64:
			n.Port = int(pt)
		}
		out = append(out, n)
	}
	return out
}

// SubNode is a compact view of one proxy entry extracted from a subscription
// result. Used by TUI tabs that display node lists.
type SubNode struct {
	Name   string
	Proto  string
	Server string
	Port   int
}

// SubscriptionNames returns a snapshot of the current subscription names in
// store order.
func (p *Pipeline) SubscriptionNames() []store.Subscription {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]store.Subscription, len(p.store.Cfg.Subscriptions))
	copy(out, p.store.Cfg.Subscriptions)
	return out
}

// AddSubscription appends a new subscription to the store and persists.
func (p *Pipeline) AddSubscription(sub store.Subscription) error {
	p.mu.Lock()
	for _, s := range p.store.Cfg.Subscriptions {
		if s.Name == sub.Name {
			p.mu.Unlock()
			return fmt.Errorf("subscription %q already exists", sub.Name)
		}
	}
	p.store.Cfg.Subscriptions = append(p.store.Cfg.Subscriptions, sub)
	p.mu.Unlock()
	return p.store.Save()
}

// DeleteSubscription removes a subscription by name and persists.
func (p *Pipeline) DeleteSubscription(name string) error {
	p.mu.Lock()
	idx := -1
	for i, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		p.mu.Unlock()
		return fmt.Errorf("subscription %q not found", name)
	}
	p.store.Cfg.Subscriptions = append(p.store.Cfg.Subscriptions[:idx], p.store.Cfg.Subscriptions[idx+1:]...)
	delete(p.subResults, name)
	p.mu.Unlock()
	return p.store.Save()
}

// ToggleSubscriptionEnabled flips the Enabled flag for a named subscription.
func (p *Pipeline) ToggleSubscriptionEnabled(name string) error {
	p.mu.Lock()
	for i, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			p.store.Cfg.Subscriptions[i].Enabled = !s.Enabled
			p.mu.Unlock()
			return p.store.Save()
		}
	}
	p.mu.Unlock()
	return fmt.Errorf("subscription %q not found", name)
}

// SaveLocal persists localNodes + localRules back into the Store.
func (p *Pipeline) SaveLocal() error {
	p.mu.Lock()
	allNodes := p.localNodes.All()
	allRules := p.localRules.All()
	ln := make([]store.LocalNode, 0, len(allNodes))
	for _, n := range allNodes {
		ln = append(ln, store.LocalNode{
			Name:   n.Name,
			Group:  n.Group,
			Via:    n.Via,
			Proto:  n.Proto,
			Server: n.Server,
			Port:   n.Port,
			Fields: n.Fields,
		})
	}
	lr := make([]store.LocalRule, 0, len(allRules))
	for _, r := range allRules {
		lr = append(lr, store.LocalRule{Type: r.Type, Payload: r.Payload, Target: r.Target})
	}
	p.store.Cfg.LocalNodes = ln
	p.store.Cfg.LocalRules = lr
	p.mu.Unlock()
	return p.store.Save()
}

// LocalNodeGroups returns the current local-nodes-group list (copy).
func (p *Pipeline) LocalNodeGroups() []store.LocalNodeGroup {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]store.LocalNodeGroup, len(p.store.Cfg.LocalNodeGroups))
	copy(out, p.store.Cfg.LocalNodeGroups)
	return out
}

// AddLocalGroup creates a new empty local-nodes group. Returns error if a
// group with the same name already exists.
func (p *Pipeline) AddLocalGroup(name string) error {
	if name == "" {
		return fmt.Errorf("local group name required")
	}
	p.mu.Lock()
	// (cannot defer Unlock: store.Save acquires its own lock and we must not hold p.mu across I/O)
	for _, g := range p.store.Cfg.LocalNodeGroups {
		if g.Name == name {
			p.mu.Unlock()
			return fmt.Errorf("local group %q already exists", name)
		}
	}
	p.store.Cfg.LocalNodeGroups = append(p.store.Cfg.LocalNodeGroups, store.LocalNodeGroup{
		Name: name, Enabled: true,
	})
	p.mu.Unlock()
	return p.store.Save()
}

// DeleteLocalGroup removes a group. Returns error if the group still has
// nodes (caller must mv them or pass force=true to delete cascadingly).
func (p *Pipeline) DeleteLocalGroup(name string, force bool) error {
	p.mu.Lock()
	// (cannot defer Unlock: store.Save acquires its own lock and we must not hold p.mu across I/O)
	// Check the in-memory manager — it's the authoritative source for nodes
	// that haven't been SaveLocal()-ed yet, which the store slice misses.
	hasNodes := false
	for _, n := range p.localNodes.All() {
		if n.Group == name {
			hasNodes = true
			break
		}
	}
	if hasNodes && !force {
		p.mu.Unlock()
		return fmt.Errorf("local group %q is not empty (use force to delete with nodes)", name)
	}
	idx := -1
	for i, g := range p.store.Cfg.LocalNodeGroups {
		if g.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		p.mu.Unlock()
		return fmt.Errorf("local group %q not found", name)
	}
	p.store.Cfg.LocalNodeGroups = append(p.store.Cfg.LocalNodeGroups[:idx], p.store.Cfg.LocalNodeGroups[idx+1:]...)
	if force {
		filtered := make([]store.LocalNode, 0, len(p.store.Cfg.LocalNodes))
		for _, n := range p.store.Cfg.LocalNodes {
			if n.Group != name {
				filtered = append(filtered, n)
			}
		}
		p.store.Cfg.LocalNodes = filtered
		nodes := make([]localnodes.Node, 0, len(filtered))
		for _, n := range filtered {
			nodes = append(nodes, localnodes.Node{
				Name: n.Name, Group: n.Group, Via: n.Via, Proto: n.Proto,
				Server: n.Server, Port: n.Port, Fields: n.Fields,
			})
		}
		p.localNodes.Load(nodes)
	}
	// Non-force path: group was empty, nothing in p.localNodes to reload.
	p.mu.Unlock()
	return p.store.Save()
}

// ToggleLocalGroupEnabled flips the Enabled flag.
func (p *Pipeline) ToggleLocalGroupEnabled(name string) error {
	p.mu.Lock()
	// (cannot defer Unlock: store.Save acquires its own lock and we must not hold p.mu across I/O)
	for i, g := range p.store.Cfg.LocalNodeGroups {
		if g.Name == name {
			p.store.Cfg.LocalNodeGroups[i].Enabled = !g.Enabled
			p.mu.Unlock()
			return p.store.Save()
		}
	}
	p.mu.Unlock()
	return fmt.Errorf("local group %q not found", name)
}

// RenameLocalGroup renames a group and migrates every node's Group field.
// NOTE: if store.Save() fails after the in-memory rename, the manager's
// view temporarily diverges from disk (in-memory shows newName, disk still
// has oldName). Callers must trigger a full reload — or restart vpnkit —
// to recover consistent state. The same invariant applies to all five
// Pipeline mutation methods; rename has the largest surface (touches
// LocalNodeGroups + every LocalNode in the group) so the gap is widest here.
func (p *Pipeline) RenameLocalGroup(oldName, newName string) error {
	if newName == "" {
		return fmt.Errorf("new name required")
	}
	if oldName == newName {
		return nil
	}
	p.mu.Lock()
	// (cannot defer Unlock: store.Save acquires its own lock and we must not hold p.mu across I/O)
	for _, g := range p.store.Cfg.LocalNodeGroups {
		if g.Name == newName {
			p.mu.Unlock()
			return fmt.Errorf("local group %q already exists", newName)
		}
	}
	idx := -1
	for i, g := range p.store.Cfg.LocalNodeGroups {
		if g.Name == oldName {
			idx = i
			break
		}
	}
	if idx < 0 {
		p.mu.Unlock()
		return fmt.Errorf("local group %q not found", oldName)
	}
	p.store.Cfg.LocalNodeGroups[idx].Name = newName
	for i, n := range p.store.Cfg.LocalNodes {
		if n.Group == oldName {
			p.store.Cfg.LocalNodes[i].Group = newName
		}
	}
	nodes := make([]localnodes.Node, 0, len(p.store.Cfg.LocalNodes))
	for _, n := range p.store.Cfg.LocalNodes {
		nodes = append(nodes, localnodes.Node{
			Name: n.Name, Group: n.Group, Via: n.Via, Proto: n.Proto,
			Server: n.Server, Port: n.Port, Fields: n.Fields,
		})
	}
	p.localNodes.Load(nodes)
	p.mu.Unlock()
	return p.store.Save()
}
