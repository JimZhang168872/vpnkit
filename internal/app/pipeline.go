package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"vpnkit/internal/assembler"
	"vpnkit/internal/config"
	"vpnkit/internal/extensions"
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
	extensionsPath string

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
func NewPipeline(st *store.Store, configYAMLPath, extensionsPath string) *Pipeline {
	pl := &Pipeline{
		store:          st,
		configYAMLPath: configYAMLPath,
		extensionsPath: extensionsPath,
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
		out[i] = localnodes.Node{Name: x.Name, Proto: x.Proto, Server: x.Server, Port: x.Port, Fields: x.Fields}
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
	localGroup := groups.NewLocalNodesGroup("local", p.localNodes)
	// extensions.Load returns (Extensions{}, nil) for a missing file, so the
	// blank identifier is safe for that case. Non-nil errors indicate parse
	// failures (corrupt TOML), which we surface to stderr rather than silently
	// assembling without user-defined extensions.
	ext, err := extensions.Load(p.extensionsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vpnkit: extensions load failed (using none): %v\n", err)
	}
	cfg := p.store.Cfg
	p.mu.Unlock()

	bytes_, err := assembler.Assemble(assembler.Input{
		Mode:             assembler.Mode(cfg.Mode),
		GlobalTarget:     cfg.GlobalTarget,
		Subscriptions:    subs,
		LocalNodes:       localGroup,
		LocalRules:       p.localRules.All(),
		Extensions:       ext,
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

// SaveLocal persists localNodes + localRules back into the Store.
func (p *Pipeline) SaveLocal() error {
	p.mu.Lock()
	allNodes := p.localNodes.All()
	allRules := p.localRules.All()
	ln := make([]store.LocalNode, 0, len(allNodes))
	for _, n := range allNodes {
		ln = append(ln, store.LocalNode{Name: n.Name, Proto: n.Proto, Server: n.Server, Port: n.Port, Fields: n.Fields})
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
