package main

import (
	"fmt"
	"os"
	"strings"

	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

// dispatchTarget implements `vpnkit target [<member>]`.
//
// Valid <member> values (anything else is rejected as a typo guard):
//   - "DIRECT", "REJECT" — mihomo built-ins
//   - "<sourceName>" — matches an enabled subscription or local-node group
//   - "<sourceName>-auto" — that source's url-test group
//   - "<sourceName>:<nodeName>" — a specific node within a source
//
// Pre-rc.7 this accepted ANY string ("../../etc/passwd", "", emoji garbage)
// and persisted it as-is, breaking 🚀 Proxy emission at Assemble time.
func dispatchTarget(args []string) {
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("%v", err)
	}
	if len(args) == 0 {
		fmt.Println(st.Cfg.GlobalTarget)
		return
	}
	target := args[0]
	if err := validateTarget(st, target); err != nil {
		dieUserErr("%v", err)
	}
	st.Cfg.GlobalTarget = target
	if err := st.Save(); err != nil {
		dieRuntime("%v", err)
	}
	fmt.Fprintf(os.Stdout, "✅ global_target → %s\n", target)
}

// validateTarget checks that `target` resolves to something the assembler
// will actually understand. See dispatchTarget docstring for accepted
// shapes. Empty string and unknown source names are rejected.
func validateTarget(st *store.Store, target string) error {
	if target == "" {
		return fmt.Errorf("global_target cannot be empty (use DIRECT, a source name, <source>-auto, or <source>:<node>)")
	}
	if target == "DIRECT" || target == "REJECT" {
		return nil
	}
	base := target
	if strings.HasSuffix(target, "-auto") {
		base = strings.TrimSuffix(target, "-auto")
	}
	if i := strings.Index(target, ":"); i > 0 {
		base = target[:i]
	}
	for _, s := range st.Cfg.Subscriptions {
		if s.Name == base {
			if !s.Enabled {
				return fmt.Errorf("subscription %q is disabled — enable it first (`vpnkit subs enable %s`) or pick a different target", base, base)
			}
			return nil
		}
	}
	for _, g := range st.Cfg.LocalNodeGroups {
		if g.Name == base {
			if !g.Enabled {
				return fmt.Errorf("local group %q is disabled — enable it first (`vpnkit local-groups enable %s`) or pick a different target", base, base)
			}
			return nil
		}
	}
	return fmt.Errorf("global_target %q doesn't match any source, DIRECT, REJECT, <name>-auto, or <name>:<node>", target)
}
