package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"vpnkit/internal/app"
	"vpnkit/internal/env"
	"vpnkit/internal/paths"
)

// version, commit, date are overridden at build time via -ldflags
//   -X main.version=... -X main.commit=... -X main.date=...
// (set by GoReleaser; defaults below for source builds).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			runVersion()
			return
		case "env":
			runEnv(os.Args[2:])
			return
		case "status":
			dispatchStatus(os.Args[2:])
			return
		case "ip":
			dispatchIP(os.Args[2:])
			return
		case "mode":
			dispatchMode(os.Args[2:])
			return
		case "groups":
			dispatchGroups(os.Args[2:])
			return
		case "nodes":
			dispatchNodes(os.Args[2:])
			return
		case "use":
			dispatchUse(os.Args[2:])
			return
		}
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "vpnkit:", err)
		os.Exit(1)
	}
}

func runVersion() {
	short := commit
	if len(short) > 7 {
		short = short[:7]
	}
	fmt.Printf("vpnkit %s  (commit %s, built %s)\n", version, short, date)
	p := paths.Resolve()
	if info, err := os.Stat(p.MihomoBinary()); err == nil {
		fmt.Printf("mihomo binary: %s (%d bytes)\n", p.MihomoBinary(), info.Size())
	} else {
		fmt.Println("mihomo binary: not installed")
	}
}

func runEnv(args []string) {
	fs := flag.NewFlagSet("env", flag.ExitOnError)
	shell := fs.String("shell", os.Getenv("SHELL"), "shell flavor: bash, zsh, or fish")
	noProxy := fs.String("no-proxy", "localhost,127.0.0.1,::1", "comma-separated no_proxy")
	unset := fs.Bool("unset", false, "emit unset/erase commands instead of export/set")
	_ = fs.Parse(args)

	flavor := "bash"
	switch {
	case *shell == "" || strings.Contains(*shell, "bash"):
		flavor = "bash"
	case strings.Contains(*shell, "zsh"):
		flavor = "zsh"
	case strings.Contains(*shell, "fish"):
		flavor = "fish"
	}

	port := 7890 // Phase 1: mixed-port hardcoded in skeleton; Phase 2 will plumb through store.
	out := env.Render(env.Options{Shell: flavor, Port: port, NoProxy: *noProxy, Unset: *unset})
	fmt.Print(out)
}

func dispatchStatus(args []string) {
	jsonOut, _ := parseFlags(args)
	c, st, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit status: %v", err)
	}
	if err := runStatus(os.Stdout, c, st, jsonOut); err != nil {
		dieRuntime("vpnkit status: %v", err)
	}
}

func dispatchIP(args []string) {
	jsonOut, _ := parseFlags(args)
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit ip: %v", err)
	}
	if err := runIP(os.Stdout, c, "", jsonOut); err != nil {
		dieRuntime("vpnkit ip: %v", err)
	}
}

func dispatchMode(args []string) {
	jsonOut, rest := parseFlags(args)
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit mode: %v", err)
	}
	if err := runMode(os.Stdout, c, rest, jsonOut); err != nil {
		dieUserErr("vpnkit mode: %v", err)
	}
}

func dispatchGroups(args []string) {
	jsonOut, _ := parseFlags(args)
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit groups: %v", err)
	}
	if err := runGroups(os.Stdout, c, jsonOut); err != nil {
		dieRuntime("vpnkit groups: %v", err)
	}
}

func dispatchNodes(args []string) {
	jsonOut, rest := parseFlags(args)
	if len(rest) < 1 {
		dieUserErr("vpnkit nodes: usage: vpnkit nodes <group> [--json]")
	}
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit nodes: %v", err)
	}
	if err := runNodes(os.Stdout, c, rest[0], jsonOut); err != nil {
		dieUserErr("vpnkit nodes: %v", err)
	}
}

func dispatchUse(args []string) {
	jsonOut, rest := parseFlags(args)
	if len(rest) < 2 {
		dieUserErr("vpnkit use: usage: vpnkit use <group> <node> [--json]")
	}
	c, _, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit use: %v", err)
	}
	if err := runUse(os.Stdout, c, rest[0], rest[1], jsonOut); err != nil {
		dieUserErr("vpnkit use: %v", err)
	}
}
