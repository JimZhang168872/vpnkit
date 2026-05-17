// Package groups models the unit at which vpnkit aggregates proxies and
// rules for assembly. A Group is either a Subscription (remote yaml) or a
// LocalNodes set (hand-entered).
package groups

import (
	"vpnkit/internal/localrules"
	"vpnkit/internal/subscription/proto"
)

// Kind identifies the source type of a Group.
type Kind int

const (
	KindSubscription Kind = iota + 1
	KindLocalNodes
)

// Group is the common contract assembler consumes.
type Group interface {
	Name() string
	Kind() Kind
	Enabled() bool
	Proxies() []proto.Proxy
	Rules() []localrules.Rule // nil when the group has no own rules
}
