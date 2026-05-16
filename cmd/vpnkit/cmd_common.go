package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"vpnkit/internal/api"
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

// dieUserErr writes to stderr and exits 1.
func dieUserErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// dieRuntime writes to stderr and exits 2.
func dieRuntime(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
