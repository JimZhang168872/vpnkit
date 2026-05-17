package app

import (
	"context"
	"fmt"
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
	subResults map[string]*subscription.Result // by subscription name; absent = not yet fetched
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
// the result. Returns the node count.
func (p *Pipeline) RefreshSubscription(ctx context.Context, name string) (int, error) {
	p.mu.Lock()
	var sub *store.Subscription
	for i := range p.store.Cfg.Subscriptions {
		if p.store.Cfg.Subscriptions[i].Name == name {
			sub = &p.store.Cfg.Subscriptions[i]
			break
		}
	}
	p.mu.Unlock()
	if sub == nil {
		return 0, fmt.Errorf("subscription %q not found", name)
	}
	body, err := subscription.Fetch(ctx, sub.URL, sub.UserAgent)
	if err != nil {
		return 0, err
	}
	res, err := subscription.Convert(body)
	if err != nil {
		return 0, err
	}
	p.mu.Lock()
	p.subResults[name] = &res
	sub.LastUpdated = time.Now()
	sub.NodeCount = len(res.Proxies)
	p.mu.Unlock()
	_ = p.store.Save()
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
	ext, _ := extensions.Load(p.extensionsPath)
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
	ln := make([]store.LocalNode, 0)
	for _, n := range p.localNodes.All() {
		ln = append(ln, store.LocalNode{Name: n.Name, Proto: n.Proto, Server: n.Server, Port: n.Port, Fields: n.Fields})
	}
	lr := make([]store.LocalRule, 0)
	for _, r := range p.localRules.All() {
		lr = append(lr, store.LocalRule{Type: r.Type, Payload: r.Payload, Target: r.Target})
	}
	p.store.Cfg.LocalNodes = ln
	p.store.Cfg.LocalRules = lr
	p.mu.Unlock()
	return p.store.Save()
}
