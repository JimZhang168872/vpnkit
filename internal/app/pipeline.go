package app

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
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

	// Fall back to "loyalsoldier" when the store doesn't pin a template —
	// it's the same default config.BuildSkeleton uses, and matching it here
	// keeps rc.5+ Assemble emissions identical to the bootstrap-time skeleton
	// in terms of what RULE-SETs mihomo sees.
	tmpl := cfg.LegacyRuleTemplate
	if tmpl == "" {
		tmpl = "loyalsoldier"
	}
	bytes_, err := assembler.Assemble(assembler.Input{
		Mode:             assembler.Mode(cfg.Mode),
		ActiveSource:     cfg.ActiveSource,
		GlobalTarget:     cfg.GlobalTarget,
		Subscriptions:    subs,
		LocalGroups:      localGroups,
		LocalRules:       p.localRules.All(),
		MixedPort:        cfg.MixedPort,
		ControllerPort:   cfg.ControllerPort,
		ControllerSecret: cfg.ControllerSecret,
		ProxyUser:        cfg.ProxyUser,
		ProxyPass:        cfg.ProxyPass,
		RuleTemplate:     tmpl,
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
//
// If this is the user's first proxy source AND GlobalTarget is still the
// safe-default "DIRECT", auto-set GlobalTarget to the new subscription's
// `-auto` url-test group. Reasoning: a brand-new vpnkit install has no
// proxies, so DIRECT is the only sane default. Once the user adds a
// subscription they almost certainly want unmatched traffic to flow
// through it — without this nudge, they'd have to run `vpnkit target` or
// pick a member in the Groups tab on every new install.
func (p *Pipeline) AddSubscription(sub store.Subscription) error {
	p.mu.Lock()
	for _, s := range p.store.Cfg.Subscriptions {
		if s.Name == sub.Name {
			p.mu.Unlock()
			return fmt.Errorf("subscription %q already exists", sub.Name)
		}
	}
	p.store.Cfg.Subscriptions = append(p.store.Cfg.Subscriptions, sub)
	if p.store.Cfg.GlobalTarget == "DIRECT" && firstProxySource(p.store) == sub.Name {
		p.store.Cfg.GlobalTarget = sub.Name + "-auto"
	}
	// First-source rc.7 nudge: if no ActiveSource yet AND the new sub is
	// enabled, take it. Adding a `--disabled` sub or one that's flagged
	// off must NOT claim ActiveSource — Assemble would then point
	// 🚀 Proxy at a name that isn't in proxy-groups → degrade to DIRECT.
	if p.store.Cfg.ActiveSource == "" && sub.Enabled {
		p.store.Cfg.ActiveSource = sub.Name
	}
	p.mu.Unlock()
	return p.store.Save()
}

// SetActiveSource swaps the routing source. The name must match an
// existing enabled subscription or local-node group; otherwise the swap
// is rejected so the user gets a clear error rather than silently
// breaking routing. Caller is expected to follow up with Assemble() +
// reload to push the new config to mihomo.
func (p *Pipeline) SetActiveSource(name string) error {
	p.mu.Lock()
	// Match the established pattern in AddSubscription / AddLocalGroup:
	// release p.mu BEFORE calling p.store.Save() so we never hold one
	// mutex across the I/O that acquires another. Using `defer` here
	// would hold p.mu through Save(), and any future code that locks
	// s.mu and then needs p.mu would deadlock (classic ABBA setup —
	// no concrete trigger today, but cheap to keep the locking
	// discipline consistent across all mutating methods).
	found := false
	for _, s := range p.store.Cfg.Subscriptions {
		if s.Enabled && s.Name == name {
			found = true
			break
		}
	}
	if !found {
		for _, g := range p.store.Cfg.LocalNodeGroups {
			if g.Enabled && g.Name == name {
				found = true
				break
			}
		}
	}
	if !found {
		p.mu.Unlock()
		return fmt.Errorf("active source %q is not an enabled subscription or local group", name)
	}
	p.store.Cfg.ActiveSource = name
	p.store.Cfg.GlobalTarget = name + "-auto"
	p.mu.Unlock()
	return p.store.Save()
}

// ActiveSource returns the currently active source name (for display /
// CLI output). May return "" if the store is bootstrap-empty.
func (p *Pipeline) ActiveSource() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.Cfg.ActiveSource
}

// SetMode persists the routing mode ("rule" / "global" / "direct")
// under p.mu so concurrent reads from Assemble see a consistent value.
// Without this, the Settings → Routing TUI tab mutated store.Cfg.Mode
// on the bubbletea goroutine while a background Assemble() running
// under p.mu read the same field — race-detector flag and torn state
// possible.
//
// Validates against the canonical set; invalid values are rejected
// rather than silently fall-through to ModeRule behavior in emitRules.
// The current TUI hardcodes the three values, but PipelineFace is a
// public surface and future callers (CLI, scripted automation, future
// TUI bugs) shouldn't be able to poison the store with garbage.
func (p *Pipeline) SetMode(mode string) error {
	switch mode {
	case "rule", "global", "direct":
	default:
		return fmt.Errorf("invalid mode %q: must be rule, global, or direct", mode)
	}
	p.mu.Lock()
	p.store.Cfg.Mode = mode
	p.mu.Unlock()
	return p.store.Save()
}

// RegenerateControllerSecret rolls a fresh 32-char hex token into
// store.Cfg.ControllerSecret under p.mu. Mihomo doesn't pick it up
// until the next service restart — caller's responsibility to surface
// that nuance to the user. Same concurrency reasoning as SetMode.
func (p *Pipeline) RegenerateControllerSecret() error {
	buf := make([]byte, 16)
	if _, err := cryptorand.Read(buf); err != nil {
		return fmt.Errorf("rand: %w", err)
	}
	p.mu.Lock()
	p.store.Cfg.ControllerSecret = hex.EncodeToString(buf)
	p.mu.Unlock()
	return p.store.Save()
}

// Mode returns the current routing mode under p.mu. Used by routing
// sub-page in place of direct store.Cfg.Mode read which races with
// Pipeline mutations.
func (p *Pipeline) Mode() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.Cfg.Mode
}

// SourceKind labels `name` as "subscription", "local", or "(unknown)".
// Callers needing both the kind label and concurrency-safe reads should
// use this rather than poking at store.Cfg directly — the latter races
// with any concurrent mutation through Pipeline.
func (p *Pipeline) SourceKind(name string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			return "subscription"
		}
	}
	for _, g := range p.store.Cfg.LocalNodeGroups {
		if g.Name == name {
			return "local"
		}
	}
	return "(unknown)"
}

// firstProxySource returns the name of the first ENABLED proxy source —
// subscription preferred, then local-nodes-group, in store insertion
// order. Used by AddSubscription/AddLocalGroup to detect "is this the
// very first usable one?" for the GlobalTarget / ActiveSource auto-nudge.
// Caller holds p.mu.
//
// Earlier versions returned the first item unconditionally (no Enabled
// check), which meant a disabled subscription at index 0 followed by
// the just-added new sub at index 1 returned the disabled one's name —
// the comparison `firstProxySource(st) == sub.Name` failed and the
// nudge never fired. We always want to point routing at something the
// user can actually use, so skip disabled entries.
func firstProxySource(st *store.Store) string {
	for _, s := range st.Cfg.Subscriptions {
		if s.Enabled {
			return s.Name
		}
	}
	for _, g := range st.Cfg.LocalNodeGroups {
		if g.Enabled {
			return g.Name
		}
	}
	return ""
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
	// Clear ActiveSource if we just deleted the active one. Leaving a
	// stale ActiveSource here would make the next Assemble fall through
	// topProxyMembersFor's "name doesn't match anything enabled" branch
	// → 🚀 Proxy contains only DIRECT, and the user sees "all traffic
	// goes direct" with no flash explaining why. Empty value triggers
	// firstEnabledSourceName fallback in the assembler.
	if p.store.Cfg.ActiveSource == name {
		p.store.Cfg.ActiveSource = ""
	}
	p.mu.Unlock()
	return p.store.Save()
}

// SetSubscriptionEnabled sets the Enabled flag idempotently (true sets
// enabled, false sets disabled — unlike Toggle which flips). The CLI
// `subs enable` / `subs disable` use this so running disable on an
// already-disabled sub is a no-op, not an accidental re-enable.
//
// Clears ActiveSource if disabling the currently-active source. Same
// reasoning as DeleteSubscription.
func (p *Pipeline) SetSubscriptionEnabled(name string, enabled bool) error {
	p.mu.Lock()
	for i, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			p.store.Cfg.Subscriptions[i].Enabled = enabled
			if !enabled && p.store.Cfg.ActiveSource == name {
				p.store.Cfg.ActiveSource = ""
			}
			p.mu.Unlock()
			return p.store.Save()
		}
	}
	p.mu.Unlock()
	return fmt.Errorf("subscription %q not found", name)
}

// ToggleSubscriptionEnabled flips the Enabled flag for a named subscription.
func (p *Pipeline) ToggleSubscriptionEnabled(name string) error {
	p.mu.Lock()
	for i, s := range p.store.Cfg.Subscriptions {
		if s.Name == name {
			p.store.Cfg.Subscriptions[i].Enabled = !s.Enabled
			// If we just disabled the active source, clear ActiveSource so
			// 🚀 Proxy doesn't end up referencing a disabled group. Same
			// reasoning as DeleteSubscription.
			if !p.store.Cfg.Subscriptions[i].Enabled && p.store.Cfg.ActiveSource == name {
				p.store.Cfg.ActiveSource = ""
			}
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
	// Same first-proxy-source nudge as AddSubscription: if the user had
	// no proxies and GlobalTarget was the safe DIRECT default, point it
	// at this new local group's -auto so unmatched traffic uses it.
	if p.store.Cfg.GlobalTarget == "DIRECT" && firstProxySource(p.store) == name {
		p.store.Cfg.GlobalTarget = name + "-auto"
	}
	// rc.7 ActiveSource nudge — same intent as the GlobalTarget bump.
	if p.store.Cfg.ActiveSource == "" {
		p.store.Cfg.ActiveSource = name
	}
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
	// Clear ActiveSource if we just removed it — see DeleteSubscription
	// for why (stale ActiveSource → 🚀 Proxy degrades to DIRECT-only).
	if p.store.Cfg.ActiveSource == name {
		p.store.Cfg.ActiveSource = ""
	}
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
			if !p.store.Cfg.LocalNodeGroups[i].Enabled && p.store.Cfg.ActiveSource == name {
				p.store.Cfg.ActiveSource = ""
			}
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
