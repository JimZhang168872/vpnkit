package groups

import (
	"strings"

	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription"
	"vpnkit/internal/subscription/proto"
)

type subscriptionGroup struct {
	name    string
	enabled bool
	result  *subscription.Result
}

// NewSubscriptionGroup wraps a fetched+converted subscription.Result so the
// assembler sees it through the Group interface. The result must be non-nil
// (caller responsibility); pass enabled=false to short-circuit emission.
func NewSubscriptionGroup(name string, enabled bool, res *subscription.Result) Group {
	return &subscriptionGroup{name: name, enabled: enabled, result: res}
}

func (g *subscriptionGroup) Name() string         { return g.name }
func (g *subscriptionGroup) Kind() Kind           { return KindSubscription }
func (g *subscriptionGroup) Enabled() bool        { return g.enabled }
func (g *subscriptionGroup) Proxies() []proto.Proxy {
	if g.result == nil {
		return nil
	}
	return g.result.Proxies
}

// Rules extracts the "rules:" key from a clash-style subscription Raw. Each
// line is parsed into a localrules.Rule. Lines we can't parse (non-string,
// 1-part) are skipped — subscriptions sometimes contain mihomo-only or older
// formats. NOTE: rule Type is NOT validated against localrules.validTypes
// here; that responsibility belongs to the assembler, so that mihomo-only
// types unknown to this package can still pass through if a downstream
// consumer (such as a future schema) accepts them.
func (g *subscriptionGroup) Rules() []localrules.Rule {
	if g.result == nil || g.result.Raw == nil {
		return nil
	}
	rawRules, ok := g.result.Raw["rules"].([]any)
	if !ok {
		return nil
	}
	out := make([]localrules.Rule, 0, len(rawRules))
	for _, line := range rawRules {
		s, ok := line.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(s, ",", 3)
		switch len(parts) {
		case 2: // MATCH,target — no payload
			out = append(out, localrules.Rule{Type: parts[0], Target: parts[1]})
		case 3:
			out = append(out, localrules.Rule{Type: parts[0], Payload: parts[1], Target: parts[2]})
		}
	}
	return out
}
