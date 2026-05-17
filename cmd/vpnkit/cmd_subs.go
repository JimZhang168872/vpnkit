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
		fs := flag.NewFlagSet("subs add", flag.ExitOnError)
		ua := fs.String("ua", "", "user-agent")
		_ = fs.Parse(rest)
		if fs.NArg() < 2 {
			dieUserErr("usage: vpnkit subs add <name> <url> [--ua=...]")
		}
		if err := runSubsAdd(st, fs.Arg(0), fs.Arg(1), *ua); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	case "rm", "remove":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs rm <name>")
		}
		if err := runSubsRm(st, rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	case "enable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs enable <name>")
		}
		if err := runSubsToggle(st, rest[0], true); err != nil {
			dieUserErr("%v", err)
		}
		_ = st.Save()
	case "disable":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit subs disable <name>")
		}
		if err := runSubsToggle(st, rest[0], false); err != nil {
			dieUserErr("%v", err)
		}
		_ = st.Save()
	case "update":
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		pl := app.NewPipeline(st, p.MihomoConfigFile(), p.VpnkitConfig+"/extensions.toml")
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
	default:
		dieUserErr("vpnkit subs: unknown verb %q", sub)
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
	for _, s := range st.Cfg.Subscriptions {
		if s.Name == name {
			return fmt.Errorf("subscription %q already exists", name)
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
