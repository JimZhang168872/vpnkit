package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vpnkit/internal/extensions"
	"vpnkit/internal/paths"
)

// dispatchChain routes `vpnkit chain <subcommand>`.
func dispatchChain(args []string) {
	if len(args) < 1 {
		dieUserErr("vpnkit chain: usage: vpnkit chain <ls|set|unset> ...")
	}
	path := extensionsPath()
	switch args[0] {
	case "ls":
		fs := flag.NewFlagSet("chain ls", flag.ExitOnError)
		jsonOut := fs.Bool("json", false, "JSON output")
		_ = fs.Parse(args[1:])
		if err := runChainLs(os.Stdout, path, *jsonOut); err != nil {
			dieRuntime("vpnkit chain ls: %v", err)
		}
	case "set":
		if len(args) != 3 {
			dieUserErr("vpnkit chain set: usage: vpnkit chain set <node> <via>")
		}
		if err := runChainSet(os.Stdout, path, args[1], args[2]); err != nil {
			dieUserErr("vpnkit chain set: %v", err)
		}
	case "unset":
		if len(args) != 2 {
			dieUserErr("vpnkit chain unset: usage: vpnkit chain unset <node>")
		}
		if err := runChainUnset(os.Stdout, path, args[1]); err != nil {
			dieUserErr("vpnkit chain unset: %v", err)
		}
	default:
		dieUserErr("vpnkit chain: unknown subcommand %q", args[0])
	}
}

// extensionsPath returns the canonical path ~/.config/vpnkit/extensions.toml.
func extensionsPath() string {
	p := paths.Resolve()
	return filepath.Join(filepath.Dir(p.VpnkitConfigFile()), "extensions.toml")
}

func runChainLs(out io.Writer, path string, jsonOut bool) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	if jsonOut {
		return json.NewEncoder(out).Encode(ext.Chains)
	}
	if len(ext.Chains) == 0 {
		fmt.Fprintln(out, "no chains configured")
		return nil
	}
	for _, c := range ext.Chains {
		fmt.Fprintf(out, "%s → %s\n", c.Node, c.Via)
	}
	return nil
}

func runChainSet(out io.Writer, path, node, via string) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	replaced := false
	for i, c := range ext.Chains {
		if c.Node == node {
			ext.Chains[i].Via = via
			replaced = true
			break
		}
	}
	if !replaced {
		ext.Chains = append(ext.Chains, extensions.Chain{Node: node, Via: via})
	}
	if err := extensions.Validate(ext); err != nil {
		return err
	}
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	verb := "added"
	if replaced {
		verb = "updated"
	}
	fmt.Fprintf(out, "%s: %s → %s\n", verb, node, via)
	return nil
}

func runChainUnset(out io.Writer, path, node string) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	next := ext.Chains[:0]
	removed := false
	for _, c := range ext.Chains {
		if c.Node == node {
			removed = true
			continue
		}
		next = append(next, c)
	}
	if !removed {
		fmt.Fprintf(out, "no chain for %s\n", node)
		return nil
	}
	ext.Chains = next
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	fmt.Fprintf(out, "removed chain for %s\n", node)
	return nil
}
