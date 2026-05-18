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
	"vpnkit/internal/store"
)

func dispatchSubs(args []string) {
	if len(args) == 0 {
		dieUserErr("vpnkit subs: usage: vpnkit subs <list|add|rm|enable|disable|update>")
	}
	sub, rest := args[0], args[1:]
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
		// Pre-extract --ua so it works in any argv position. flag.Parse
		// stops at the first non-flag positional, which means
		// `subs add foo url --ua=X` silently drops --ua, and
		// `subs add foo --ua=X url` even stores `--ua=X` as the URL.
		// Both are confirmed user traps from QA. Strip the flag pair
		// out before parsing positionals.
		ua, posArgs := extractUAFlag(rest)
		if len(posArgs) < 2 {
			dieUserErr("usage: vpnkit subs add <name> <url> [--ua=...]")
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
		if err := pl.SetSubscriptionEnabled(rest[0], true); err != nil {
			dieUserErr("%v", err)
		}
		mutated = true
	case "disable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs disable <name>")
		}
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
			fmt.Printf("✅ %s — %d nodes\n", n, count)
		}
		if len(errs) > 0 {
			dieRuntime("%v", errors.Join(errs...))
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

// reservedSourceNames cannot be used as a subscription / local-group name
// because they're mihomo built-in policy targets — colliding with them
// produces ambiguous routing in the assembled config.yaml.
var reservedSourceNames = map[string]bool{
	"DIRECT":      true,
	"REJECT":      true,
	"REJECT-DROP": true,
	"PASS":        true,
	"COMPATIBLE":  true,
	"GLOBAL":      true,
	"🚀 Proxy":   true,
	"🎯 Direct":  true,
	"🛑 Reject":  true,
}

// validateSourceName rejects names that would corrupt the routing
// namespace: empty, whitespace-only, or matching a mihomo built-in.
// Subscriptions and local-node groups share this namespace (both emit
// `<name>` and `<name>-auto` proxy-groups), so cross-collisions are
// rejected at the call site, not here.
func validateSourceName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name cannot be empty or whitespace")
	}
	if reservedSourceNames[name] {
		return fmt.Errorf("name %q is reserved by mihomo — pick a different one", name)
	}
	return nil
}

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
