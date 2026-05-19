package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"vpnkit/internal/app"
	"vpnkit/internal/paths"
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
	pl := app.NewPipeline(st, p.MihomoConfigFile(), p.VpnkitCache)
	if len(positional) == 0 {
		runActiveShow(os.Stdout, pl, wantJSON)
		return
	}
	name := positional[0]
	if err := runActiveSet(os.Stdout, pl, name, wantJSON); err != nil {
		dieUserErr("%v", err)
	}
	applyMutation(pl)
}

// runActiveShow prints the current active source (text or JSON). Routes
// all reads through the Pipeline so they happen under p.mu instead of
// touching store.Cfg directly (which would race with concurrent
// mutations on the same pointer).
func runActiveShow(out io.Writer, pl *app.Pipeline, wantJSON bool) {
	name := pl.ActiveSource()
	if wantJSON {
		_ = json.NewEncoder(out).Encode(map[string]any{
			"active_source": name,
			"kind":          pl.SourceKind(name),
		})
		return
	}
	if name == "" {
		fmt.Fprintln(out, "(none — vpnkit will pick the first enabled source on next assemble)")
		return
	}
	fmt.Fprintf(out, "%s  (%s)\n", name, pl.SourceKind(name))
}

// runActiveSet swaps the active source via Pipeline.SetActiveSource and
// writes a confirmation. Doesn't reload mihomo — that's the caller's
// responsibility (applyMutation in the CLI dispatcher).
func runActiveSet(out io.Writer, pl *app.Pipeline, name string, wantJSON bool) error {
	if err := pl.SetActiveSource(name); err != nil {
		return err
	}
	if wantJSON {
		_ = json.NewEncoder(out).Encode(map[string]string{
			"active_source": name,
			"kind":          pl.SourceKind(name),
		})
		return nil
	}
	fmt.Fprintf(out, "✅ active_source → %s (%s)\n", name, pl.SourceKind(name))
	return nil
}

