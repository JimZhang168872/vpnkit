package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"vpnkit/internal/app"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

func dispatchLocalGroups(args []string) {
	if len(args) == 0 {
		dieUserErr("vpnkit local-groups: usage: vpnkit local-groups <list|add|rm|enable|disable|rename>")
	}
	sub, rest := args[0], args[1:]
	if sub != "list" && sub != "ls" {
		rejectJSONOnMutation("vpnkit local-groups "+sub, rest)
	}
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit local-groups: %v", err)
	}
	pl := app.NewPipeline(st, p.MihomoConfigFile())
	mutated := false
	switch sub {
	case "list", "ls":
		jsonOut := false
		fs := flag.NewFlagSet("local-groups list", flag.ExitOnError)
		fs.BoolVar(&jsonOut, "json", false, "")
		_ = fs.Parse(rest)
		runLocalGroupsList(os.Stdout, st, jsonOut)
	case "add":
		if len(rest) != 1 {
			dieUserErr("usage: vpnkit local-groups add <name>")
		}
		name := rest[0]
		if err := validateSourceName(name); err != nil {
			dieUserErr("%v", err)
		}
		// Cross-namespace collision check: subs and local-groups share
		// the routing namespace. A sub already named "shared" plus a
		// local-group "shared" would emit a duplicate-key mihomo config.
		for _, s := range st.Cfg.Subscriptions {
			if s.Name == name {
				dieUserErr("name %q already used by a subscription — sources share the routing namespace, pick a different name", name)
			}
		}
		if err := pl.AddLocalGroup(name); err != nil {
			dieUserErr("%v", err)
		}
		fmt.Printf("✅ created local group %q\n", name)
		mutated = true
	case "rm", "remove":
		fs := flag.NewFlagSet("local-groups rm", flag.ExitOnError)
		force := fs.Bool("force", false, "delete even if the group has nodes (cascade)")
		_ = fs.Parse(rest)
		if fs.NArg() < 1 {
			dieUserErr("usage: vpnkit local-groups rm <name> [--force]")
		}
		if err := pl.DeleteLocalGroup(fs.Arg(0), *force); err != nil {
			dieUserErr("%v", err)
		}
		fmt.Printf("✅ removed local group %q\n", fs.Arg(0))
		mutated = true
	case "enable", "disable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-groups %s <name>", sub)
		}
		rejectExtraArgs("vpnkit local-groups "+sub, rest, 1)
		current := false
		found := false
		for _, g := range st.Cfg.LocalNodeGroups {
			if g.Name == rest[0] {
				current = g.Enabled
				found = true
				break
			}
		}
		if !found {
			dieUserErr("local group %q not found", rest[0])
		}
		want := sub == "enable"
		if current == want {
			fmt.Printf("✅ local group %q already %sd\n", rest[0], sub)
			return
		}
		if err := pl.ToggleLocalGroupEnabled(rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		fmt.Printf("✅ %sd local group %q\n", sub, rest[0])
		mutated = true
	case "rename":
		if len(rest) < 2 {
			dieUserErr("usage: vpnkit local-groups rename <old> <new>")
		}
		rejectExtraArgs("vpnkit local-groups rename", rest, 2)
		// Same validation as `local-groups add` — pre-rc.7 rename was a
		// loophole around every guard (reserved names, cross-namespace).
		if err := validateSourceName(rest[1]); err != nil {
			dieUserErr("%v", err)
		}
		if err := pl.RenameLocalGroup(rest[0], rest[1]); err != nil {
			dieUserErr("%v", err)
		}
		fmt.Printf("✅ renamed local group %q → %q\n", rest[0], rest[1])
		mutated = true
	default:
		dieUserErr("vpnkit local-groups: unknown verb %q", sub)
	}
	if mutated {
		applyMutation(pl)
	}
}

func runLocalGroupsList(out io.Writer, st *store.Store, jsonOut bool) {
	// Pre-compute node counts so both text and JSON paths share them.
	// Pre-rc.7 the JSON path emitted the raw LocalNodeGroup struct which
	// lacks the count — users scripting against --json got no way to see
	// group population without a separate `local-nodes list` query.
	type groupOut struct {
		Name      string `json:"name"`
		Enabled   bool   `json:"enabled"`
		NodeCount int    `json:"node_count"`
	}
	rows := make([]groupOut, 0, len(st.Cfg.LocalNodeGroups))
	for _, g := range st.Cfg.LocalNodeGroups {
		count := 0
		for _, n := range st.Cfg.LocalNodes {
			if n.Group == g.Name {
				count++
			}
		}
		rows = append(rows, groupOut{Name: g.Name, Enabled: g.Enabled, NodeCount: count})
	}
	if jsonOut {
		_ = json.NewEncoder(out).Encode(rows)
		return
	}
	for _, r := range rows {
		mark := "✅"
		if !r.Enabled {
			mark = "  "
		}
		fmt.Fprintf(out, "%s  %-20s  %d nodes\n", mark, r.Name, r.NodeCount)
	}
}
