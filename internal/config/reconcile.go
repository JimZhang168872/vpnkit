package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecurityFields are the mihomo config keys vpnkit owns and forces onto every
// generated config.yaml. They are the single line of defense against other
// users on the same host using the proxy without authorization.
type SecurityFields struct {
	MixedPort        int
	ControllerPort   int
	ControllerSecret string
	ProxyUser        string
	ProxyPass        string
	// ReleaseMirror, when non-empty, prefixes every github.com URL in the
	// generated geox-url block. Empty → defaults to jsdelivr CDN.
	ReleaseMirror string
}

// EnsureSecurityFields rewrites the security-related keys in `path`'s YAML to
// match what's in the store. Subscription-owned keys (proxies, proxy-groups,
// rules, geox-url, log-level, mode, …) are left untouched. Returns
// changed=true if the file was rewritten.
//
// Solves the upgrade gap: a user who installed pre-v0.7.0 already has a
// config.yaml with no `authentication:` block, and bootstrap's "generate only
// when missing" rule would never inject one. This function runs every launch
// so security fields stay in sync with the store as source of truth.
func EnsureSecurityFields(path string, sf SecurityFields) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("config.yaml not found: %s", path)
		}
		return false, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parse config.yaml: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}

	want := map[string]any{
		"mixed-port":          sf.MixedPort,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", sf.ControllerPort),
		"secret":              sf.ControllerSecret,
		"allow-lan":           false,
		"bind-address":        "127.0.0.1",
	}
	if sf.ProxyUser != "" && sf.ProxyPass != "" {
		want["authentication"] = []any{sf.ProxyUser + ":" + sf.ProxyPass}
	}

	changed := false
	for k, v := range want {
		if !equal(doc[k], v) {
			doc[k] = v
			changed = true
		}
	}

	// Backfill geox-url only when missing — users may have customized it via
	// patch.yaml and we should preserve their choice. Without geox-url mihomo
	// would default to downloading from github.com, which times out inside
	// the GFW on first boot and prevents the controller from ever listening.
	if _, has := doc["geox-url"]; !has {
		geox := mihomoGeoxURL(sf.ReleaseMirror)
		any_ := map[string]any{}
		for k, v := range geox {
			any_[k] = v
		}
		doc["geox-url"] = any_
		changed = true
	}

	if !changed {
		return false, nil
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return false, err
	}
	return true, AtomicWrite(path, []byte(unescapeEmoji(string(out))), 0o600)
}

// unescapeEmoji reverses yaml.v3's habit of emitting non-ASCII strings as
// \UXXXXXXXX escapes. mihomo reads either form correctly, but users (and
// vpnkit's own subscription writer) operate on the literal-emoji form, so
// stay consistent.
func unescapeEmoji(s string) string {
	return emojiReplacer.Replace(s)
}

var emojiReplacer = strings.NewReplacer(
	`\U0001F680`, "🚀",
	`\U0001F3AF`, "🎯",
	`\U0001F6D1`, "🛑",
	`\U000267B`, "♻️",
)

// equal compares two values for YAML-semantic equality. yaml.Unmarshal turns
// numbers into `int` and strings into `string`; for our small set of typed
// keys, reflect.DeepEqual after normalizing the auth-list is enough.
func equal(a, b any) bool {
	// Normalize authentication: yaml gives []any of strings; we write []any too.
	if as, ok := a.([]any); ok {
		if bs, ok := b.([]any); ok {
			if len(as) != len(bs) {
				return false
			}
			for i := range as {
				if !reflect.DeepEqual(fmt.Sprint(as[i]), fmt.Sprint(bs[i])) {
					return false
				}
			}
			return true
		}
	}
	return reflect.DeepEqual(a, b)
}
