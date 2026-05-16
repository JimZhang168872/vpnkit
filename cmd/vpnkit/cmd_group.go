package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"vpnkit/internal/extensions"
)

type groupAddOpts struct {
	Name      string
	Type      string
	Proxies   []string
	URL       string
	Interval  int
	Tolerance int
}

func dispatchGroup(args []string) {
	if len(args) < 1 {
		dieUserErr("vpnkit group: usage: vpnkit group <ls|add|rm> ...")
	}
	path := extensionsPath()
	switch args[0] {
	case "ls":
		fs := flag.NewFlagSet("group ls", flag.ExitOnError)
		jsonOut := fs.Bool("json", false, "JSON output")
		_ = fs.Parse(args[1:])
		if err := runGroupLs(os.Stdout, path, *jsonOut); err != nil {
			dieRuntime("vpnkit group ls: %v", err)
		}
	case "add":
		fs := flag.NewFlagSet("group add", flag.ExitOnError)
		typ := fs.String("type", "select", "group type: select|url-test|fallback|load-balance|relay")
		proxies := fs.String("proxies", "", "comma-separated proxy names")
		url := fs.String("url", "", "(optional) test URL")
		interval := fs.Int("interval", 0, "(optional) test interval seconds")
		tolerance := fs.Int("tolerance", 0, "(optional) tolerance ms")
		_ = fs.Parse(args[1:])
		if fs.NArg() < 1 {
			dieUserErr("vpnkit group add: usage: vpnkit group add <name> --type <t> --proxies a,b,c [...]")
		}
		opts := groupAddOpts{
			Name:      fs.Arg(0),
			Type:      *typ,
			Proxies:   splitCSVCmd(*proxies),
			URL:       *url,
			Interval:  *interval,
			Tolerance: *tolerance,
		}
		if err := runGroupAdd(os.Stdout, path, opts); err != nil {
			dieUserErr("vpnkit group add: %v", err)
		}
	case "rm":
		if len(args) != 2 {
			dieUserErr("vpnkit group rm: usage: vpnkit group rm <name>")
		}
		if err := runGroupRm(os.Stdout, path, args[1]); err != nil {
			dieUserErr("vpnkit group rm: %v", err)
		}
	default:
		dieUserErr("vpnkit group: unknown subcommand %q", args[0])
	}
}

func splitCSVCmd(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func runGroupLs(out io.Writer, path string, jsonOut bool) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	if jsonOut {
		return json.NewEncoder(out).Encode(ext.Groups)
	}
	if len(ext.Groups) == 0 {
		fmt.Fprintln(out, "no groups configured")
		return nil
	}
	for _, g := range ext.Groups {
		fmt.Fprintf(out, "%s [%s] %s\n", g.Name, g.Type, strings.Join(g.Proxies, ","))
	}
	return nil
}

func runGroupAdd(out io.Writer, path string, opts groupAddOpts) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	for i, g := range ext.Groups {
		if g.Name == opts.Name {
			ext.Groups[i] = extensions.Group{
				Name: opts.Name, Type: opts.Type, Proxies: opts.Proxies,
				URL: opts.URL, Interval: opts.Interval, Tolerance: opts.Tolerance,
			}
			if err := extensions.Validate(ext); err != nil {
				return err
			}
			if err := extensions.Save(path, ext); err != nil {
				return err
			}
			fmt.Fprintf(out, "updated: %s\n", opts.Name)
			return nil
		}
	}
	ext.Groups = append(ext.Groups, extensions.Group{
		Name: opts.Name, Type: opts.Type, Proxies: opts.Proxies,
		URL: opts.URL, Interval: opts.Interval, Tolerance: opts.Tolerance,
	})
	if err := extensions.Validate(ext); err != nil {
		return err
	}
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	fmt.Fprintf(out, "added: %s\n", opts.Name)
	return nil
}

func runGroupRm(out io.Writer, path, name string) error {
	ext, err := extensions.Load(path)
	if err != nil {
		return err
	}
	next := ext.Groups[:0]
	removed := false
	for _, g := range ext.Groups {
		if g.Name == name {
			removed = true
			continue
		}
		next = append(next, g)
	}
	if !removed {
		fmt.Fprintf(out, "no group %s\n", name)
		return nil
	}
	ext.Groups = next
	if err := extensions.Save(path, ext); err != nil {
		return err
	}
	fmt.Fprintf(out, "removed group: %s\n", name)
	return nil
}
