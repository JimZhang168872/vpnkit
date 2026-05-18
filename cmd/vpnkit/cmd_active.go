package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"vpnkit/internal/app"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// dispatchActive implements `vpnkit active [<source-name>] [--json]`.
//
// No name → print the current ActiveSource and whether it's a sub or a
// local group. With a name → set it (must match an enabled source) and
// reassemble + reload mihomo so the new routing takes effect immediately.
func dispatchActive(args []string) {
	wantJSON := false
	positional := args[:0]
	for _, a := range args {
		if a == "--json" {
			wantJSON = true
			continue
		}
		positional = append(positional, a)
	}
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("%v", err)
	}
	if len(positional) == 0 {
		runActiveShow(os.Stdout, st, wantJSON)
		return
	}
	name := positional[0]
	pl := app.NewPipeline(st, p.MihomoConfigFile())
	if err := runActiveSet(os.Stdout, pl, st, name, wantJSON); err != nil {
		dieUserErr("%v", err)
	}
	applyMutation(pl)
}

// runActiveShow prints the current active source (text or JSON). Pure I/O
// against the passed writer + store. Testable.
func runActiveShow(out io.Writer, st *store.Store, wantJSON bool) {
	name := st.Cfg.ActiveSource
	if wantJSON {
		_ = json.NewEncoder(out).Encode(map[string]any{
			"active_source": name,
			"kind":          activeKind(st, name),
		})
		return
	}
	if name == "" {
		fmt.Fprintln(out, "(none — vpnkit will pick the first enabled source on next assemble)")
		return
	}
	fmt.Fprintf(out, "%s  (%s)\n", name, activeKind(st, name))
}

// runActiveSet swaps the active source via Pipeline.SetActiveSource and
// writes a confirmation. Doesn't reload mihomo — that's the caller's
// responsibility (applyMutation in the CLI dispatcher). Testable.
func runActiveSet(out io.Writer, pl *app.Pipeline, st *store.Store, name string, wantJSON bool) error {
	if err := pl.SetActiveSource(name); err != nil {
		return err
	}
	if wantJSON {
		_ = json.NewEncoder(out).Encode(map[string]string{
			"active_source": name,
			"kind":          activeKind(st, name),
		})
		return nil
	}
	fmt.Fprintf(out, "✅ active_source → %s (%s)\n", name, activeKind(st, name))
	return nil
}

// activeKind labels a source name as "subscription" / "local" so users
// can tell what they're pointing at. Returns "(unknown)" when the name
// references a source that no longer exists (e.g. user deleted the sub
// but the active_source field is stale).
func activeKind(st *store.Store, name string) string {
	for _, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			return "subscription"
		}
	}
	for _, g := range st.Cfg.LocalNodeGroups {
		if g.Name == name {
			return "local"
		}
	}
	return "(unknown)"
}
