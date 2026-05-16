package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"vpnkit/internal/api"
)

func runGroups(out io.Writer, c *api.Client, jsonOut bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proxies, err := c.GetProxies(ctx)
	if err != nil {
		return fmt.Errorf("get proxies: %w", err)
	}

	type entry struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Now     string `json:"now"`
		Members int    `json:"members"`
	}
	var rows []entry
	for name, info := range proxies {
		if !isUserSelectableType(info.Type) {
			continue
		}
		if isBuiltinGroup(name) {
			continue
		}
		if len(info.All) == 0 {
			continue
		}
		rows = append(rows, entry{Name: name, Type: info.Type, Now: info.Now, Members: len(info.All)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	if jsonOut {
		return writeJSON(out, rows)
	}
	tbl := make([][]string, 0, len(rows))
	for _, e := range rows {
		tbl = append(tbl, []string{e.Name, e.Type, e.Now, fmt.Sprintf("%d", e.Members)})
	}
	renderTable(out, []string{"GROUP", "TYPE", "CURRENT", "MEMBERS"}, tbl)
	return nil
}

// isBuiltinGroup filters out system groups that users cannot select.
func isBuiltinGroup(name string) bool {
	switch name {
	case "DIRECT", "REJECT", "GLOBAL":
		return true
	}
	return false
}
