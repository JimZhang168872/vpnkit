package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"vpnkit/internal/localnodes"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

func dispatchLocalNodes(args []string) {
	if len(args) == 0 {
		dieUserErr("vpnkit local-nodes: usage: vpnkit local-nodes <list|add|rm|edit>")
	}
	sub, rest := args[0], args[1:]
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit local-nodes: %v", err)
	}
	switch sub {
	case "list", "ls":
		jsonOut := false
		for _, a := range rest {
			if a == "--json" {
				jsonOut = true
			}
		}
		if err := runLocalNodesList(os.Stdout, st, jsonOut); err != nil {
			dieRuntime("%v", err)
		}
	case "add":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-nodes add <uri>")
		}
		if err := runLocalNodesAdd(st, rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	case "rm", "remove":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-nodes rm <name>")
		}
		if err := runLocalNodesRm(st, rest[0]); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	case "edit":
		if len(rest) < 2 {
			dieUserErr("usage: vpnkit local-nodes edit <name> <key=val>...")
		}
		if err := runLocalNodesEdit(st, rest[0], rest[1:]); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
	default:
		dieUserErr("vpnkit local-nodes: unknown verb %q", sub)
	}
}

func runLocalNodesList(out io.Writer, st *store.Store, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(out).Encode(st.Cfg.LocalNodes)
	}
	for _, n := range st.Cfg.LocalNodes {
		fmt.Fprintf(out, "%-20s  %-10s  %-20s  %d\n", n.Name, n.Proto, n.Server, n.Port)
	}
	return nil
}

func runLocalNodesAdd(st *store.Store, uri string) error {
	n, err := localnodes.ParseURI(uri)
	if err != nil {
		return fmt.Errorf("parse uri: %w", err)
	}
	// Check duplicate name.
	for _, existing := range st.Cfg.LocalNodes {
		if existing.Name == n.Name {
			return fmt.Errorf("local node %q already exists", n.Name)
		}
	}
	st.Cfg.LocalNodes = append(st.Cfg.LocalNodes, store.LocalNode{
		Name:   n.Name,
		Proto:  n.Proto,
		Server: n.Server,
		Port:   n.Port,
		Fields: n.Fields,
	})
	return nil
}

func runLocalNodesRm(st *store.Store, name string) error {
	for i, n := range st.Cfg.LocalNodes {
		if n.Name == name {
			st.Cfg.LocalNodes = append(st.Cfg.LocalNodes[:i], st.Cfg.LocalNodes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("local node %q not found", name)
}

// runLocalNodesEdit updates named node fields. Special top-level keys
// (proto/server/port/name) update their struct fields directly; all other
// keys update Fields map entries.
func runLocalNodesEdit(st *store.Store, name string, pairs []string) error {
	for i, n := range st.Cfg.LocalNodes {
		if n.Name == name {
			if st.Cfg.LocalNodes[i].Fields == nil {
				st.Cfg.LocalNodes[i].Fields = make(map[string]any)
			}
			for _, kv := range pairs {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid key=val: %q", kv)
				}
				key, val := parts[0], parts[1]
				switch key {
				case "name":
					st.Cfg.LocalNodes[i].Name = val
				case "proto":
					st.Cfg.LocalNodes[i].Proto = val
				case "server":
					st.Cfg.LocalNodes[i].Server = val
				case "port":
					p, err := strconv.Atoi(val)
					if err != nil {
						return fmt.Errorf("invalid port %q: %w", val, err)
					}
					st.Cfg.LocalNodes[i].Port = p
				default:
					st.Cfg.LocalNodes[i].Fields[key] = val
				}
			}
			return nil
		}
	}
	return fmt.Errorf("local node %q not found", name)
}

var _ = errors.New // silence unused import if any future change drops usage
