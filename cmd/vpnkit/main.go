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

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			runVersion()
			return
		case "env":
			runEnv(os.Args[2:])
			return
		}
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "vpnkit:", err)
		os.Exit(1)
	}
}

func runVersion() {
	fmt.Printf("vpnkit %s\n", version)
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
