package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"vpnkit/internal/app"
	"vpnkit/internal/paths"
	"vpnkit/internal/sources"
	"vpnkit/internal/store"
)

func dispatchSubs(args []string) {
	if len(args) == 0 {
		dieUserErr("vpnkit subs: usage: vpnkit subs <list|add|rm|enable|disable|update>")
	}
	sub, rest := args[0], args[1:]
	// Reject --json on mutation subverbs (list/ls are the only read ones).
	// Pre-rc.7 each subverb's `if len(rest) < N` check would either eat
	// --json as a positional or produce a confusing "too many args" message.
	if sub != "list" && sub != "ls" {
		rejectJSONOnMutation("vpnkit subs "+sub, rest)
	}
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit subs: %v", err)
	}
	pl := app.NewPipeline(st, p.MihomoConfigFile())
	mutated := false
	switch sub {
	case "list", "ls":
		jsonOut := false
		fs := flag.NewFlagSet("subs list", flag.ExitOnError)
		fs.BoolVar(&jsonOut, "json", false, "")
		_ = fs.Parse(rest)
		if err := runSubsList(os.Stdout, st, jsonOut); err != nil {
			dieRuntime("%v", err)
		}
	case "add":
		ua, posArgs := extractUAFlag(rest)
		if len(posArgs) < 2 {
			dieUserErr("usage: vpnkit subs add <name> <url> [--ua=...]")
		}
		// Reject extra positional args — silently dropping them
		// (pre-rc.7) made `subs add foo URL garbage1 garbage2` rc=0
		// with garbage1/garbage2 invisibly discarded. Users typing
		// shell-quoted things would assume their input was honored.
		if len(posArgs) > 2 {
			dieUserErr("subs add takes exactly 2 positional args (name, url); got %d: %v", len(posArgs), posArgs)
		}
		if err := runSubsAdd(st, posArgs[0], posArgs[1], ua); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
		mutated = true
	case "rm", "remove":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs rm <name>")
		}
		rejectExtraArgs("vpnkit subs rm", rest, 1)
		// Route through Pipeline.DeleteSubscription so the
		// ActiveSource-clearing side-effect fires (rc.7+: stale
		// ActiveSource pointing at a deleted sub would degrade 🚀 Proxy
		// to [DIRECT] silently).
		if err := pl.DeleteSubscription(rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		mutated = true
	case "enable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs enable <name>")
		}
		rejectExtraArgs("vpnkit subs enable", rest, 1)
		if err := pl.SetSubscriptionEnabled(rest[0], true); err != nil {
			dieUserErr("%v", err)
		}
		mutated = true
	case "disable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs disable <name>")
		}
		rejectExtraArgs("vpnkit subs disable", rest, 1)
		// Idempotent set (not toggle) — disabling an already-disabled
		// sub is a no-op, NOT an accidental re-enable. Also clears
		// ActiveSource if it was pointing at this sub.
		if err := pl.SetSubscriptionEnabled(rest[0], false); err != nil {
			dieUserErr("%v", err)
		}
		mutated = true
	case "update":
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		names := rest
		if len(names) == 0 {
			for _, s := range st.Cfg.Subscriptions {
				names = append(names, s.Name)
			}
		}
		var errs []error
		for _, n := range names {
			count, err := pl.RefreshSubscription(ctx, n)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", n, err))
				continue
			}
			if count == 0 {
				// A 0-node refresh is almost always a sign of malformed
				// YAML, an HTML error page, or a feed that requires a
				// different User-Agent (doggygosubs returns dummy nodes
				// for non-mihomo UAs — historical bug). Surface loud.
				fmt.Fprintf(os.Stderr, "⚠️  %s — 0 nodes (malformed feed? UA gated? check the URL with curl)\n", n)
				continue
			}
			fmt.Printf("✅ %s — %d nodes\n", n, count)
		}
		if len(errs) > 0 {
			// "name not found" is a user error (typo, deleted sub) —
			// match `subs rm` and `subs disable` which return rc=1 for
			// the same condition. Runtime errors (network, fetch
			// failure mid-flight) are bundled into the same message;
			// either way rc=1 is more correct than rc=2 here because
			// `errors.Join` flattens both kinds.
			dieUserErr("%v", errors.Join(errs...))
		}
		mutated = true
	default:
		dieUserErr("vpnkit subs: unknown verb %q", sub)
	}
	if mutated {
		applyMutation(pl)
	}
}

func runSubsList(out io.Writer, st *store.Store, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(out).Encode(st.Cfg.Subscriptions)
	}
	for _, s := range st.Cfg.Subscriptions {
		mark := "✅"
		if !s.Enabled {
			mark = "  "
		}
		fmt.Fprintf(out, "%s  %-20s  %3d nodes  %s\n", mark, s.Name, s.NodeCount, s.URL)
	}
	return nil
}

func runSubsAdd(st *store.Store, name, url, ua string) error {
	if name == "" || url == "" {
		return errors.New("name and url required")
	}
	if err := validateSourceName(name); err != nil {
		return err
	}
	if err := validateSubURL(url); err != nil {
		return err
	}
	for _, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			return fmt.Errorf("subscription %q already exists", name)
		}
	}
	for _, g := range st.Cfg.LocalNodeGroups {
		if g.Name == name {
			return fmt.Errorf("name %q already used by a local-node group — sources share the routing namespace, pick a different name", name)
		}
	}
	st.Cfg.Subscriptions = append(st.Cfg.Subscriptions, store.Subscription{
		Name: name, URL: url, UserAgent: ua, Enabled: true,
	})
	return nil
}

func runSubsRm(st *store.Store, name string) error {
	for i, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			st.Cfg.Subscriptions = append(st.Cfg.Subscriptions[:i], st.Cfg.Subscriptions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

func runSubsToggle(st *store.Store, name string, enabled bool) error {
	for i, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			st.Cfg.Subscriptions[i].Enabled = enabled
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

var _ = strings.TrimSpace // silence unused import if any future change drops usage

// maxSourceNameLen is re-exported here for the local-nodes edit path
// which uses a slightly different validation (allows `:` because node
// names historically used colons). Source-name validation itself lives
// in internal/sources.
const maxSourceNameLen = sources.MaxNameLen

// validSubSchemes are the URL schemes a subscription URL can use.
// HTTP(S) for hosted clash YAML feeds; the single-URI schemes
// (ss/vmess/...) are accepted so users can paste a one-off proxy URI
// directly via `subs add`.
var validSubSchemes = map[string]bool{
	"http":      true,
	"https":     true,
	"ss":        true,
	"ssr":       true,
	"vmess":     true,
	"vless":     true,
	"trojan":    true,
	"hysteria":  true,
	"hysteria2": true,
	"hy2":       true,
	"tuic":      true,
}

// validateSubURL rejects URLs that wouldn't actually fetch a subscription:
// unknown schemes (file://, javascript:, gopher://, etc.) AND any URL
// with embedded control characters. The Fetch path silently treats
// non-HTTP URLs as proxy literals, so confusing schemes used to leak
// through without warning.
func validateSubURL(rawURL string) error {
	for _, c := range rawURL {
		if (c < 0x20 && c != 0) || c == 0x7f {
			return fmt.Errorf("url contains control character (0x%02x)", c)
		}
	}
	// Allow-list by scheme prefix. Cheap parse rather than url.Parse
	// because we want to accept ss://... etc., which url.Parse handles
	// but produces less obvious schemes for.
	for sch := range validSubSchemes {
		if strings.HasPrefix(rawURL, sch+"://") {
			return nil
		}
	}
	return fmt.Errorf("url scheme not allowed (got %q) — must be one of: http(s), ss, vmess, vless, trojan, hysteria(2), hy2, tuic, ssr", schemeOf(rawURL))
}

// schemeOf extracts the scheme portion of `url://...` for error messages.
func schemeOf(url string) string {
	if i := strings.Index(url, "://"); i > 0 {
		return url[:i]
	}
	return url
}

// validateSourceName delegates to internal/sources so CLI and TUI share
// one validation surface. Keep this wrapper so existing call sites don't
// have to change.
func validateSourceName(name string) error { return sources.ValidateName(name) }

// extractUAFlag walks `args` and pulls out a `--ua=VALUE` or `--ua VALUE`
// pair from any position. Returns the UA value (or "" if not found) and
// the remaining positional args. Why hand-rolled instead of flag.Parse:
// the stdlib parser stops at the first non-flag positional, so flags
// after positionals get silently dropped or misinterpreted as the next
// positional. That's a real user-confirmed trap (`subs add foo url --ua=X`
// stored --ua=X as the URL once tickets had two positionals consumed).
func extractUAFlag(args []string) (ua string, positional []string) {
	positional = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--ua" && i+1 < len(args):
			ua = args[i+1]
			i++
		case strings.HasPrefix(a, "--ua="):
			ua = a[len("--ua="):]
		case a == "-ua" && i+1 < len(args):
			ua = args[i+1]
			i++
		case strings.HasPrefix(a, "-ua="):
			ua = a[len("-ua="):]
		default:
			positional = append(positional, a)
		}
	}
	return
}
