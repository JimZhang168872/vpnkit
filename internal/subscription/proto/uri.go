package proto

import (
	"fmt"
	"net/url"
	"strings"
)

// uriParts holds the structural pieces of a single-URI proxy after a lenient
// split that tolerates "/" inside the userinfo (which RFC 3986 forbids in
// authority but which real-world subscription URLs contain anyway).
//
// Compared to url.Parse, the difference is exactly the userinfo/host boundary:
// we use the LAST '@' before the query/fragment, instead of letting "/" close
// the authority section prematurely.
type uriParts struct {
	UserInfo string // raw (still URL-encoded) text before '@'; "" if absent
	Host     string // hostname (no port)
	Port     int
	Query    url.Values
	Fragment string // already percent-decoded
}

// splitProxyURI parses a "scheme://userinfo@host:port?query#fragment" URI
// without delegating authority parsing to net/url, so that "/" inside the
// userinfo does not corrupt host/port/userinfo extraction.
func splitProxyURI(uri string) (uriParts, error) {
	_, rest, err := schemeOf(uri)
	if err != nil {
		return uriParts{}, err
	}
	var out uriParts

	if i := strings.Index(rest, "#"); i >= 0 {
		out.Fragment, _ = url.QueryUnescape(rest[i+1:])
		rest = rest[:i]
	}
	if i := strings.Index(rest, "?"); i >= 0 {
		out.Query, _ = url.ParseQuery(rest[i+1:])
		rest = rest[:i]
	}

	// LAST '@' is the real userinfo/host boundary — any earlier '@' belongs
	// to the userinfo. Same trick ss.go already uses.
	hostPort := rest
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		out.UserInfo = rest[:at]
		hostPort = rest[at+1:]
	}
	if hostPort == "" {
		return uriParts{}, fmt.Errorf("missing host")
	}
	host, port, err := splitHostPort(hostPort)
	if err != nil {
		return uriParts{}, err
	}
	out.Host = host
	out.Port = port
	return out, nil
}
