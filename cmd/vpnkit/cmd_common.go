package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"vpnkit/internal/api"
	"vpnkit/internal/app"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// loadClient reads vpnkit's config.toml and returns an api.Client + Store.
func loadClient() (*api.Client, *store.Store, error) {
	p := paths.Resolve()
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return nil, nil, fmt.Errorf("load store: %w", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", st.Cfg.ControllerPort)
	return api.New(url, st.Cfg.ControllerSecret), st, nil
}

// rejectJSONOnMutation aborts with a clear user error when args contains
// `--json`. Mutation verbs (subs add/rm, local-rules add/rm/move,
// local-nodes add/rm/edit/mv, local-groups add/rm/enable/disable/rename)
// don't have a defined JSON output shape; pre-rc.7 each verb either
// silently dropped --json into positional args or produced confusing
// "too many positional args" errors. Call this once at the top of every
// mutation dispatcher.
func rejectJSONOnMutation(verbName string, args []string) {
	for _, a := range args {
		if a == "--json" {
			dieUserErr("%s: --json is only supported on read verbs (list/ls/show); use `%s` followed by a read verb to see JSON output", verbName, verbName)
		}
	}
}

// rejectExtraArgs aborts when args has more positional args than `want`.
// Pre-rc.7 most mutation dispatchers silently dropped extras —
// `subs add foo URL garbage1` returned rc=0 with garbage1 invisibly
// discarded. Now every mutation verb that takes a fixed positional
// count calls this to catch the typo loud.
func rejectExtraArgs(verbName string, args []string, want int) {
	if len(args) > want {
		dieUserErr("%s: takes exactly %d positional arg(s); got %d: %v", verbName, want, len(args), args)
	}
}

// lockIfMutating is a wrapper that only acquires the store flock when
// args[0] indicates a mutation. Read-only subverbs (`list`/`ls`, no-arg
// show forms of `target`/`active`/`mode`) bypass the lock so they don't
// starve when a long-running mutation holds the exclusive lock.
//
// Heuristic: if args is empty (= show form of a verb that supports it)
// OR args[0] is "list"/"ls"/"show", treat as read-only. Otherwise lock.
func lockIfMutating(args []string, fn func()) {
	if len(args) == 0 {
		fn()
		return
	}
	switch args[0] {
	case "list", "ls", "show", "--json":
		// "--json" alone (`subs --json`) means "list as JSON" for the
		// list-default verbs. Read-only.
		fn()
		return
	}
	withStoreLock(fn)
}

// withStoreLock acquires a POSIX advisory lock on the config file before
// invoking fn, and releases it after. Used to serialize CLI mutators so
// concurrent `vpnkit subs add &` workers don't race their read-modify-
// write of config.toml. CLI mutation dispatchers (subs / local-nodes /
// local-groups / local-rules / target / mode / active / init) wrap their
// whole flow with this; read-only verbs (status / ip / env / list) skip
// it to avoid pointless contention.
//
// Failure to acquire (e.g. read-only filesystem) exits via dieRuntime so
// the caller doesn't proceed silently.
func withStoreLock(fn func()) {
	p := paths.Resolve()
	lock, err := store.AcquireLock(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit: acquire config lock: %v", err)
	}
	defer lock.Release()
	fn()
}

// applyMutation reassembles config.yaml from the current store and asks the
// running mihomo to reload. Best-effort on the reload step: if mihomo isn't
// running (`vpnkit init` before first launch, vpnkit-only operations) we
// still want the new config.yaml written so the next mihomo launch picks
// it up. The reload error is non-fatal but reported on stderr so users can
// see when their mutation didn't take effect immediately.
//
// Most mutation CLI commands (subs add/rm/update, local-nodes /
// local-groups / local-rules CRUD) should call this at the end. Without
// it, the store is updated but mihomo's running config has no idea about
// the new state, so follow-up calls (`vpnkit use`, `vpnkit test`) 404.
func applyMutation(pl *app.Pipeline) {
	if err := pl.Assemble(); err != nil {
		fmt.Fprintf(os.Stderr, "vpnkit: reassemble failed: %v\n", err)
		return
	}
	c, _, err := loadClient()
	if err != nil {
		// store unreadable would have already aborted upstream — be quiet here.
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ReloadConfig(ctx, ""); err != nil {
		// Most common reason: mihomo not running yet. New config is on disk;
		// next launch picks it up. Surface just enough so the user knows.
		fmt.Fprintf(os.Stderr, "vpnkit: mihomo reload skipped (%v) — config.yaml updated on disk\n", err)
	}
}

// parseFlags extracts a `--json` flag from args (any position).
func parseFlags(args []string) (jsonOut bool, rest []string) {
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		rest = append(rest, a)
	}
	return
}

// renderTable writes a left-aligned ASCII table to out.
func renderTable(out io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runeLen(h)
	}
	for _, row := range rows {
		for i, c := range row {
			if i >= len(widths) {
				continue
			}
			if l := runeLen(c); l > widths[i] {
				widths[i] = l
			}
		}
	}
	writeRow(out, headers, widths)
	for _, row := range rows {
		writeRow(out, row, widths)
	}
}

func writeRow(out io.Writer, cols []string, widths []int) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(out, "  ")
		}
		fmt.Fprint(out, c)
		pad := widths[i] - runeLen(c)
		for p := 0; p < pad; p++ {
			fmt.Fprint(out, " ")
		}
	}
	fmt.Fprintln(out)
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// writeJSON marshals v compactly and writes a trailing newline.
func writeJSON(out io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	_, err = out.Write([]byte("\n"))
	return err
}

// dieUserErr writes to stderr and exits 1 (user/input error).
// Overrideable in tests to avoid os.Exit.
var dieUserErr = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// dieRuntime writes to stderr and exits 2 (runtime/internal error).
// Overrideable in tests to avoid os.Exit.
var dieRuntime = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
