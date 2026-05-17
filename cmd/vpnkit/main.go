package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpnkit/internal/app"
	"vpnkit/internal/env"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// storeLoad is a thin pointer so unit tests could override later.
var storeLoad = store.Load

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
		case "--version", "-v", "version":
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
		case "init":
			dispatchInit(os.Args[2:])
			return
		case "uninstall":
			dispatchUninstall(os.Args[2:])
			return
		case "update":
			dispatchUpdate(os.Args[2:])
			return
		case "chain":
			dispatchChain(os.Args[2:])
			return
		case "group":
			dispatchGroup(os.Args[2:])
			return
		case "ext":
			dispatchExt(os.Args[2:])
			return
		case "subs":
			dispatchSubs(os.Args[2:])
			return
		case "local-nodes":
			dispatchLocalNodes(os.Args[2:])
			return
		case "local-rules":
			dispatchLocalRules(os.Args[2:])
			return
		case "target":
			dispatchTarget(os.Args[2:])
			return
		}
	}
	if err := app.Run(version); err != nil {
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
	noNetrc := fs.Bool("no-netrc", false, "skip writing ~/.netrc")
	functions := fs.Bool("functions", false, "emit proxy_on / proxy_off function defs (append once to ~/.zshrc)")
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

	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	port, user, pass := 7890, "", ""
	if err == nil {
		port = st.Cfg.MixedPort
		if port == 0 {
			port = 7890
		}
		user = st.Cfg.ProxyUser
		pass = st.Cfg.ProxyPass
	}

	out := env.Render(env.Options{
		Shell: flavor, Port: port, User: user, Pass: pass,
		NoProxy: *noProxy, Unset: *unset, Functions: *functions,
	})
	fmt.Print(out)

	if !*unset && !*functions && !*noNetrc && user != "" && pass != "" {
		if home, herr := os.UserHomeDir(); herr == nil {
			netrcPath := filepath.Join(home, ".netrc")
			_ = env.WriteNetrc(netrcPath, "127.0.0.1", user, pass)
		}
	}
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
	c, st, err := loadClient()
	if err != nil {
		dieRuntime("vpnkit ip: %v", err)
	}
	if err := runIP(os.Stdout, c, st, "", jsonOut); err != nil {
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

func dispatchInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	restore := fs.String("restore", "", "path to a profiles backup TOML to merge")
	force := fs.Bool("force", false, "back up any existing store before regenerating (use to recover from v1 → v2)")
	_ = fs.Bool("non-interactive", false, "(no-op; init is always non-interactive)")
	_ = fs.Parse(args)
	if err := runInit(os.Stdout, runInitOpts{RestorePath: *restore, Force: *force}); err != nil {
		dieRuntime("vpnkit init: %v", err)
	}
}

func dispatchUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	yes := fs.Bool("yes", false, "skip interactive confirmation")
	purge := fs.Bool("purge", false, "also delete profiles (no backup)")
	keepMihomo := fs.Bool("keep-mihomo", false, "do not delete ~/.local/bin/mihomo")
	keepProfiles := fs.Bool("keep-profiles", true, "back up profiles to --backup-dir before delete")
	backupDir := fs.String("backup-dir", "/tmp", "where to write the profiles backup")
	_ = fs.Parse(args)
	// --purge implies !keep-profiles. --keep-profiles=false (without --purge)
	// also skips the backup — same end state.
	purgeEffective := *purge || !*keepProfiles
	opts := uninstallOptions{
		Yes:        *yes,
		Purge:      purgeEffective,
		KeepMihomo: *keepMihomo,
		BackupDir:  *backupDir,
	}
	if err := runUninstall(os.Stdout, opts); err != nil {
		dieRuntime("vpnkit uninstall: %v", err)
	}
}

func dispatchUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	check := fs.Bool("check", false, "only print what's available, don't install")
	vpnkitOnly := fs.Bool("vpnkit-only", false, "only update vpnkit itself, leave mihomo alone")
	mihomoOnly := fs.Bool("mihomo-only", false, "only update mihomo core, leave vpnkit alone")
	yes := fs.Bool("yes", false, "skip interactive confirmation")
	_ = fs.Parse(args)
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit update: %v", err)
	}
	if err := runUpdate(os.Stdout, updateOptions{
		Check: *check, VpnkitOnly: *vpnkitOnly, MihomoOnly: *mihomoOnly, Yes: *yes,
	}, st, version); err != nil {
		dieRuntime("vpnkit update: %v", err)
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
