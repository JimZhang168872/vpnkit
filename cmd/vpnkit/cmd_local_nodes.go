package main

import (
	"encoding/json"
	"errors"
	"flag"
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
		dieUserErr("vpnkit local-nodes: usage: vpnkit local-nodes <list|add|rm|edit|mv>")
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
		// URI is positional arg 0; flags (--group, --via) may follow it.
		// Go's flag package stops at the first non-flag token, so we split
		// the positional URI from the remaining flag-bearing args manually.
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-nodes add <uri> [--group=<name>] [--via=<target>]")
		}
		uri := rest[0]
		flagArgs := rest[1:]
		fs := flag.NewFlagSet("local-nodes add", flag.ExitOnError)
		groupFlag := fs.String("group", "", "target local-nodes-group (default: 'local')")
		viaFlag := fs.String("via", "", "dialer-proxy target (proxy/group name)")
		_ = fs.Parse(flagArgs)
		node, err := localnodes.ParseURI(uri)
		if err != nil {
			dieUserErr("parse: %v", err)
		}
		if *groupFlag != "" {
			node.Group = *groupFlag
		} else {
			node.Group = "local"
		}
		node.Via = *viaFlag
		st.Cfg.LocalNodes = append(st.Cfg.LocalNodes, store.LocalNode{
			Name: node.Name, Group: node.Group, Via: node.Via,
			Proto: node.Proto, Server: node.Server, Port: node.Port, Fields: node.Fields,
		})
		hasGroup := false
		for _, g := range st.Cfg.LocalNodeGroups {
			if g.Name == node.Group {
				hasGroup = true
				break
			}
		}
		if !hasGroup {
			st.Cfg.LocalNodeGroups = append(st.Cfg.LocalNodeGroups, store.LocalNodeGroup{
				Name: node.Group, Enabled: true,
			})
		}
		if err := st.Save(); err != nil {
			dieRuntime("save: %v", err)
		}
		fmt.Printf("✅ added local node %s:%s\n", node.Group, node.Name)
	case "rm", "remove":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-nodes rm <node>")
		}
		group, name, ambig, ok := resolveLocalNode(st, rest[0])
		if ambig {
			dieUserErr("vpnkit: ambiguous %q — use \"<group>:<name>\"", rest[0])
		}
		if !ok {
			dieUserErr("local node %q not found", rest[0])
		}
		out := st.Cfg.LocalNodes[:0]
		for _, n := range st.Cfg.LocalNodes {
			if n.Name == name && n.Group == group {
				continue
			}
			out = append(out, n)
		}
		st.Cfg.LocalNodes = out
		if err := st.Save(); err != nil {
			dieRuntime("save: %v", err)
		}
		fmt.Printf("✅ removed %s:%s\n", group, name)
	case "mv":
		if len(rest) < 2 {
			dieUserErr("usage: vpnkit local-nodes mv <node> <new-group>")
		}
		group, name, ambig, ok := resolveLocalNode(st, rest[0])
		if ambig {
			dieUserErr("vpnkit: ambiguous %q — use \"<group>:<name>\"", rest[0])
		}
		if !ok {
			dieUserErr("local node %q not found", rest[0])
		}
		for i := range st.Cfg.LocalNodes {
			if st.Cfg.LocalNodes[i].Name == name && st.Cfg.LocalNodes[i].Group == group {
				st.Cfg.LocalNodes[i].Group = rest[1]
				break
			}
		}
		if err := st.Save(); err != nil {
			dieRuntime("save: %v", err)
		}
		fmt.Printf("✅ moved %s:%s → %s\n", group, name, rest[1])
	case "edit":
		if len(rest) < 2 {
			dieUserErr("usage: vpnkit local-nodes edit <node> key=val [...]")
		}
		group, name, ambig, ok := resolveLocalNode(st, rest[0])
		if ambig {
			dieUserErr("vpnkit: ambiguous %q — use \"<group>:<name>\"", rest[0])
		}
		if !ok {
			dieUserErr("local node %q not found", rest[0])
		}
		var target *store.LocalNode
		for i := range st.Cfg.LocalNodes {
			if st.Cfg.LocalNodes[i].Name == name && st.Cfg.LocalNodes[i].Group == group {
				target = &st.Cfg.LocalNodes[i]
				break
			}
		}
		if target == nil {
			dieUserErr("local node %q not found", rest[0])
		}
		for _, kv := range rest[1:] {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				dieUserErr("bad kv %q (want key=val)", kv)
			}
			k, v := parts[0], parts[1]
			switch k {
			case "name":
				target.Name = v
			case "group":
				target.Group = v
			case "via":
				target.Via = v
			case "proto":
				target.Proto = v
			case "server":
				target.Server = v
			case "port":
				p, err := strconv.Atoi(v)
				if err != nil {
					dieUserErr("port must be int: %v", err)
				}
				target.Port = p
			default:
				if target.Fields == nil {
					target.Fields = map[string]any{}
				}
				target.Fields[k] = v
			}
		}
		if err := st.Save(); err != nil {
			dieRuntime("save: %v", err)
		}
		fmt.Printf("✅ edited %s:%s\n", group, target.Name)
	default:
		dieUserErr("vpnkit local-nodes: unknown verb %q", sub)
	}
}

// resolveLocalNode finds a node by short name (e.g. "HK-A") or namespaced
// form (e.g. "home:HK-A"). Returns the namespaced group/name pair, plus a
// bool indicating ambiguity. Caller must dieUserErr on ambiguity.
func resolveLocalNode(st *store.Store, ref string) (group, name string, ambiguous bool, found bool) {
	if i := strings.Index(ref, ":"); i > 0 {
		return ref[:i], ref[i+1:], false, true
	}
	matches := 0
	for _, n := range st.Cfg.LocalNodes {
		if n.Name == ref {
			matches++
			group, name = n.Group, n.Name
		}
	}
	switch matches {
	case 0:
		return "", "", false, false
	case 1:
		return group, name, false, true
	default:
		return "", "", true, true
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
