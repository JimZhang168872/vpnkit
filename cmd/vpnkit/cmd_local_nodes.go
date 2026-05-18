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

	"vpnkit/internal/app"
	"vpnkit/internal/localnodes"
	"vpnkit/internal/paths"
	"vpnkit/internal/sources"
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
	pl := app.NewPipeline(st, p.MihomoConfigFile())
	mutated := false
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
		fs := flag.NewFlagSet("local-nodes add", flag.ExitOnError)
		groupFlag := fs.String("group", "", "target local-nodes-group (default: 'local')")
		viaFlag := fs.String("via", "", "dialer-proxy target (proxy/group name)")

		// flag.Parse stops at the first non-flag token. The URI may come
		// before OR after the flags, so split args into flags and the URI
		// manually based on the "--" or "://" sniff.
		var uri string
		var flagArgs []string
		for _, a := range rest {
			if strings.Contains(a, "://") {
				if uri != "" {
					dieUserErr("local-nodes add: multiple URIs in %v", rest)
				}
				uri = a
				continue
			}
			flagArgs = append(flagArgs, a)
		}
		if uri == "" {
			dieUserErr("usage: vpnkit local-nodes add <uri> [--group=<name>] [--via=<target>]")
		}
		_ = fs.Parse(flagArgs)
		node, err := localnodes.ParseURI(uri)
		if err != nil {
			dieUserErr("parse: %v", err)
		}
		// ParseURI accepts port 0 / 65536+ via net/url's lax parse —
		// reject explicitly so the assembled YAML doesn't contain
		// nodes mihomo will refuse to dial.
		if node.Port < 1 || node.Port > 65535 {
			dieUserErr("port %d out of range (must be 1-65535)", node.Port)
		}
		// Block shell metacharacters in the URI's #fragment (which
		// becomes node.Name). Users with `ss://...#$(whoami)` URIs
		// would otherwise persist an unsafe name that downstream scripts
		// might interpolate into shell commands.
		if err := sources.ValidateNodeName(node.Name); err != nil {
			dieUserErr("node name from URI fragment: %v", err)
		}
		if *groupFlag != "" {
			node.Group = *groupFlag
		} else {
			node.Group = "local"
		}
		node.Via = *viaFlag
		// Duplicate-name check by (Group, Name). Without this, adding the
		// same URI twice silently stores two entries; subsequent `rm` then
		// errors "ambiguous" and rules-emit produces duplicate proxy names
		// that mihomo rejects. The dead-code path in runLocalNodesAdd had
		// this check; the inline branch above bypassed it.
		for _, existing := range st.Cfg.LocalNodes {
			if existing.Name == node.Name && existing.Group == node.Group {
				dieUserErr("local node %q already exists in group %q", node.Name, node.Group)
			}
		}
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
		mutated = true
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
		mutated = true
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
		newGroup := rest[1]
		// Require the target group to exist explicitly. Auto-creating
		// (pre-rc.7) meant a typo in the group name silently birthed a
		// brand-new group that escaped every reserved-name / cross-
		// namespace guard — `mv n1 DIRECT` would happily create a
		// `DIRECT` local-group.
		hasGroup := false
		targetEnabled := true
		for _, g := range st.Cfg.LocalNodeGroups {
			if g.Name == newGroup {
				hasGroup = true
				targetEnabled = g.Enabled
				break
			}
		}
		if !hasGroup {
			dieUserErr("destination group %q does not exist — create it first with `vpnkit local-groups add %s`", newGroup, newGroup)
		}
		for i := range st.Cfg.LocalNodes {
			if st.Cfg.LocalNodes[i].Name == name && st.Cfg.LocalNodes[i].Group == group {
				st.Cfg.LocalNodes[i].Group = newGroup
				break
			}
		}
		// Warn (not refuse) when moving into a disabled group — node
		// becomes unroutable until the group is re-enabled. Without this
		// the user gets silent "why isn't my node showing up" later.
		if !targetEnabled {
			fmt.Fprintf(os.Stderr, "⚠️  group %q is disabled — node %q is now unroutable until you `vpnkit local-groups enable %s`\n",
				newGroup, name, newGroup)
		}
		if err := st.Save(); err != nil {
			dieRuntime("save: %v", err)
		}
		fmt.Printf("✅ moved %s:%s → %s\n", group, name, newGroup)
		mutated = true
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
		// Snapshot current (group, name) so collision checks compare
		// against the OTHER nodes, not the target itself. Renaming a
		// node to its own current name is a no-op, not a collision.
		origGroup, origName := target.Group, target.Name
		for _, kv := range rest[1:] {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				dieUserErr("bad kv %q (want key=val)", kv)
			}
			k, v := parts[0], parts[1]
			switch k {
			case "name":
				// sources.ValidateNodeName is the looser sibling of
				// ValidateName — accepts emoji/space/parens (consistent
				// with subscription feed names) but blocks shell
				// metacharacters and control chars to defend against
				// `$(whoami)` round-tripping through scripts.
				if err := sources.ValidateNodeName(v); err != nil {
					dieUserErr("%v", err)
				}
				// Duplicate (group, name) check — would otherwise create
				// two `[[local_nodes]]` blocks with the same key, leaving
				// the second one unreachable via `rm`/`edit` until the
				// user hand-edits the TOML.
				for _, n := range st.Cfg.LocalNodes {
					if n.Group == target.Group && n.Name == v && !(n.Group == origGroup && n.Name == origName) {
						dieUserErr("local node %q already exists in group %q", v, target.Group)
					}
				}
				target.Name = v
			case "group":
				// Target group must exist (same rule as `mv` — pre-rc.7
				// `edit group=` silently moved into a non-existent group
				// which would then orphan the node).
				hasGroup := false
				targetEnabled := true
				for _, g := range st.Cfg.LocalNodeGroups {
					if g.Name == v {
						hasGroup = true
						targetEnabled = g.Enabled
						break
					}
				}
				if !hasGroup {
					dieUserErr("destination group %q does not exist — create it first with `vpnkit local-groups add %s`", v, v)
				}
				// Also reject duplicate (group, name) when MOVING to a
				// group that already has a node with target.Name.
				for _, n := range st.Cfg.LocalNodes {
					if n.Group == v && n.Name == target.Name && !(n.Group == origGroup && n.Name == origName) {
						dieUserErr("local node %q already exists in group %q", target.Name, v)
					}
				}
				if !targetEnabled {
					fmt.Fprintf(os.Stderr, "⚠️  group %q is disabled — node %q is now unroutable until you `vpnkit local-groups enable %s`\n",
						v, target.Name, v)
				}
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
				if p < 1 || p > 65535 {
					dieUserErr("port %d out of range (must be 1-65535)", p)
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
		mutated = true
	default:
		dieUserErr("vpnkit local-nodes: unknown verb %q", sub)
	}
	if mutated {
		applyMutation(pl)
	}
}

// resolveLocalNode finds a node by short name (e.g. "HK-A") or namespaced
// form (e.g. "home:HK-A"). Returns the namespaced group/name pair, plus a
// bool indicating ambiguity. Caller must dieUserErr on ambiguity.
func resolveLocalNode(st *store.Store, ref string) (group, name string, ambiguous bool, found bool) {
	if i := strings.Index(ref, ":"); i > 0 {
		group, name := ref[:i], ref[i+1:]
		for _, n := range st.Cfg.LocalNodes {
			if n.Group == group && n.Name == name {
				return group, name, false, true
			}
		}
		return group, name, false, false
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
