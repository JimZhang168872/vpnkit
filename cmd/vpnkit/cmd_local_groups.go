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
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit local-groups: %v", err)
	}
	pl := app.NewPipeline(st, p.MihomoConfigFile(), p.VpnkitConfig+"/extensions.toml")
	switch sub {
	case "list", "ls":
		jsonOut := false
		fs := flag.NewFlagSet("local-groups list", flag.ExitOnError)
		fs.BoolVar(&jsonOut, "json", false, "")
		_ = fs.Parse(rest)
		runLocalGroupsList(os.Stdout, st, jsonOut)
	case "add":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-groups add <name>")
		}
		if err := pl.AddLocalGroup(rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		fmt.Printf("✅ created local group %q\n", rest[0])
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
	case "enable", "disable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-groups %s <name>", sub)
		}
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
	case "rename":
		if len(rest) < 2 {
			dieUserErr("usage: vpnkit local-groups rename <old> <new>")
		}
		if err := pl.RenameLocalGroup(rest[0], rest[1]); err != nil {
			dieUserErr("%v", err)
		}
		fmt.Printf("✅ renamed local group %q → %q\n", rest[0], rest[1])
	default:
		dieUserErr("vpnkit local-groups: unknown verb %q", sub)
	}
}

func runLocalGroupsList(out io.Writer, st *store.Store, jsonOut bool) {
	if jsonOut {
		_ = json.NewEncoder(out).Encode(st.Cfg.LocalNodeGroups)
		return
	}
	for _, g := range st.Cfg.LocalNodeGroups {
		mark := "✅"
		if !g.Enabled {
			mark = "  "
		}
		count := 0
		for _, n := range st.Cfg.LocalNodes {
			if n.Group == g.Name {
				count++
			}
		}
		fmt.Fprintf(out, "%s  %-20s  %d nodes\n", mark, g.Name, count)
	}
}
