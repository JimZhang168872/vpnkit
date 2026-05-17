package main

import (
	"fmt"
	"os"

	"vpnkit/internal/paths"
)

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
	st.Cfg.GlobalTarget = args[0]
	if err := st.Save(); err != nil {
		dieRuntime("%v", err)
	}
	fmt.Fprintf(os.Stdout, "✅ global_target → %s\n", args[0])
}
