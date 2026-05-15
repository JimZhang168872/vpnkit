// Package proto holds per-protocol parsers turning subscription URIs into
// mihomo.Proxy values (a generic map[string]any matching mihomo's YAML schema).
package proto

import (
	"fmt"
	"strings"
)

// Proxy is a mihomo proxy entry, deliberately untyped to mirror the YAML schema.
// Required keys: name, type, server, port. Other keys depend on protocol.
type Proxy map[string]any

// Parser converts a single proxy URI (e.g. "vmess://...") to a Proxy.
type Parser func(uri string) (Proxy, error)

// registered parsers, populated by per-protocol init() calls.
var registry = map[string]Parser{}

// Register adds a parser for a scheme (call from proto package init).
func Register(scheme string, p Parser) { registry[scheme] = p }

// Parse dispatches the URI to the matching scheme parser.
func Parse(uri string) (Proxy, error) {
	scheme, _, err := schemeOf(uri)
	if err != nil {
		return nil, err
	}
	parser, ok := registry[scheme]
	if !ok {
		return nil, fmt.Errorf("proto: unsupported scheme %q", scheme)
	}
	return parser(uri)
}

func schemeOf(uri string) (string, string, error) {
	i := strings.Index(uri, "://")
	if i <= 0 {
		return "", "", fmt.Errorf("proto: not a URI: %q", uri)
	}
	return uri[:i], uri[i+3:], nil
}
