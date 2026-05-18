package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"vpnkit/internal/app"
	"vpnkit/internal/localrules"
	"vpnkit/internal/paths"
	"vpnkit/internal/store"
)

func dispatchLocalRules(args []string) {
	if len(args) == 0 {
		dieUserErr("vpnkit local-rules: usage: vpnkit local-rules <list|add|rm|move>")
	}
	sub, rest := args[0], args[1:]
	if sub != "list" && sub != "ls" {
		rejectJSONOnMutation("vpnkit local-rules "+sub, rest)
	}
	p := paths.Resolve()
	st, err := storeLoad(p.VpnkitConfigFile())
	if err != nil {
		dieRuntime("vpnkit local-rules: %v", err)
	}
	pl := app.NewPipeline(st, p.MihomoConfigFile())
	mutated := false
	switch sub {
	case "list", "ls":
		jsonOut := false
		for _, a := range rest {
			if a == "--json" {
				jsonOut = true
			}
		}
		if err := runLocalRulesList(os.Stdout, st, jsonOut); err != nil {
			dieRuntime("%v", err)
		}
	case "add":
		if len(rest) < 3 {
			dieUserErr("usage: vpnkit local-rules add <type> <payload> <target>")
		}
		rejectExtraArgs("vpnkit local-rules add", rest, 3)
		if err := runLocalRulesAdd(st, rest[0], rest[1], rest[2]); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
		mutated = true
	case "rm", "remove":
		if len(rest) < 1 {
			dieUserErr("usage: vpnkit local-rules rm <idx>")
		}
		rejectExtraArgs("vpnkit local-rules rm", rest, 1)
		idx, err := strconv.Atoi(rest[0])
		if err != nil {
			dieUserErr("invalid index %q: %v", rest[0], err)
		}
		if err := runLocalRulesRm(st, idx); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
		mutated = true
	case "move":
		if len(rest) < 2 {
			dieUserErr("usage: vpnkit local-rules move <from> <to>")
		}
		rejectExtraArgs("vpnkit local-rules move", rest, 2)
		from, err := strconv.Atoi(rest[0])
		if err != nil {
			dieUserErr("invalid from index %q: %v", rest[0], err)
		}
		to, err := strconv.Atoi(rest[1])
		if err != nil {
			dieUserErr("invalid to index %q: %v", rest[1], err)
		}
		if err := runLocalRulesMove(st, from, to); err != nil {
			dieUserErr("%v", err)
		}
		if err := st.Save(); err != nil {
			dieRuntime("%v", err)
		}
		mutated = true
	default:
		dieUserErr("vpnkit local-rules: unknown verb %q", sub)
	}
	if mutated {
		applyMutation(pl)
	}
}

func runLocalRulesList(out io.Writer, st *store.Store, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(out).Encode(st.Cfg.LocalRules)
	}
	for i, r := range st.Cfg.LocalRules {
		rule := localrules.Rule{Type: r.Type, Payload: r.Payload, Target: r.Target}
		fmt.Fprintf(out, "[%d] %s\n", i, rule.Render())
	}
	return nil
}

func runLocalRulesAdd(st *store.Store, ruleType, payload, target string) error {
	r := localrules.Rule{Type: ruleType, Payload: payload, Target: target}
	if err := localrules.Validate(r); err != nil {
		return err
	}
	st.Cfg.LocalRules = append(st.Cfg.LocalRules, store.LocalRule{
		Type: ruleType, Payload: payload, Target: target,
	})
	return nil
}

func runLocalRulesRm(st *store.Store, idx int) error {
	if idx < 0 || idx >= len(st.Cfg.LocalRules) {
		return fmt.Errorf("index %d out of range (0..%d)", idx, len(st.Cfg.LocalRules)-1)
	}
	st.Cfg.LocalRules = append(st.Cfg.LocalRules[:idx], st.Cfg.LocalRules[idx+1:]...)
	return nil
}

func runLocalRulesMove(st *store.Store, from, to int) error {
	n := len(st.Cfg.LocalRules)
	if from < 0 || from >= n || to < 0 || to >= n {
		return fmt.Errorf("bad indices %d→%d (len=%d)", from, to, n)
	}
	if from == to {
		return nil
	}
	r := st.Cfg.LocalRules[from]
	st.Cfg.LocalRules = append(st.Cfg.LocalRules[:from], st.Cfg.LocalRules[from+1:]...)
	st.Cfg.LocalRules = append(st.Cfg.LocalRules[:to], append([]store.LocalRule{r}, st.Cfg.LocalRules[to:]...)...)
	return nil
}
