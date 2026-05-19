package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/config"
	"vpnkit/internal/msg"
	"vpnkit/internal/paths"
	"vpnkit/internal/portutil"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
	tabsettings "vpnkit/internal/tabs/settings"
)

// Run launches the vpnkit TUI. `version` is the current vpnkit binary
// version (from main.version / ldflags); passing empty/"dev" disables the
// startup update check. Returns the bubbletea exit error.
func Run(version string) error {
	p := paths.Resolve()
	if err := p.Ensure(); err != nil {
		return fmt.Errorf("paths: %w", err)
	}
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	// Detect service mode on first run + build the manager. Shared with
	// dispatchInit so the TUI and CLI bootstrap agree on backend selection.
	svc := NewServiceManager(p, st)
	// Reconcile ports against the local OS before profMgr captures them. Skip
	// when our mihomo is already running (its bound ports are presumably the saved ones).
	if err := reconcilePorts(svc, st, p.MihomoConfigFile()); err != nil {
		return fmt.Errorf("reconcile ports: %w", err)
	}
	client := api.New(fmt.Sprintf("http://127.0.0.1:%d", st.Cfg.ControllerPort), st.Cfg.ControllerSecret)
	// Force the security-owned keys (ports, auth, bind-address, …) into any
	// pre-existing config.yaml — e.g. an upgrade from a pre-auth version
	// where bootstrap would otherwise never regenerate. If the file is
	// absent, bootstrap will create it from scratch.
	configChanged, _ := ensureConfigSecurity(st, p.MihomoConfigFile())

	pl := NewPipeline(st, p.MihomoConfigFile())

	// Force-reassemble config.yaml from the current store on every launch.
	// Cheap (one Marshal + AtomicWrite) and protects against three classes
	// of "config drift" the user can't easily recover from manually:
	//
	//   1. Lazy store migration (e.g. GlobalTarget "🚀 Proxy" → "DIRECT"
	//      in rc.6) changed the in-memory store but the on-disk
	//      config.yaml still reflects the pre-migration state.
	//   2. A previous vpnkit version emitted a config mihomo refuses
	//      (the self-loop bug). The store is fine; the YAML needs a
	//      fresh emit from the corrected assembler.
	//   3. User hand-edited config.yaml and broke it. Reassemble
	//      overwrites with vpnkit's authoritative view from store.toml.
	//
	// Errors are logged and execution continues — bootstrap will still
	// try to start mihomo with whatever's on disk, and the user will see
	// the failure surface via service status / Dashboard.
	if err := pl.Assemble(); err != nil {
		fmt.Fprintf(os.Stderr, "vpnkit: startup reassemble failed (%v) — mihomo may load a stale config\n", err)
	}

	// Closure that subscription-update + startup-reload paths use to push config
	// changes into the live mihomo. Tries hot reload first, restarts the
	// service on any error (e.g. controller-secret drift between store.toml
	// and mihomo's in-memory boot state — silently fatal in v0.7.1).
	applyCfg := func(ctx context.Context) error {
		if err := pl.Assemble(); err != nil {
			return err
		}
		return applyConfig(ctx, client, svc)
	}

	settingsDeps := tabsettings.Deps{
		Paths:     p,
		Store:     st,
		Service:   svc,
		APIClient: client,
		Pipeline:  pl,
		Version:   version,
		ApplyFunc: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return applyCfg(ctx)
		},
	}
	model := NewModel(client, settingsDeps, applyCfg)
	model.WirePipeline(pl)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	// Shutdown signal for every background goroutine. Cancelled the
	// instant prog.Run() returns (user pressed q / Ctrl-C). Without this,
	// goroutines that hold long-lived HTTP/WebSocket connections to mihomo
	// (streamTraffic, streamConnections, streamLogs) keep spinning in
	// 2s-backoff reconnect loops AFTER the TUI exits, preventing the
	// process from terminating. The streamLogs case is especially nasty:
	// it has a `tail -F` or `journalctl -f` subprocess attached, which
	// only dies when its parent context is cancelled.
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	go func() {
		msg := MaybeBootstrap(BootstrapDeps{
			Paths:   p,
			Store:   st,
			Service: svc,
		})()
		if configChanged {
			// Derive from shutdownCtx so an early user quit doesn't leak
			// this goroutine for up to 10 seconds while applyCfg waits
			// on its own timer. prog.Send below is a documented no-op
			// after prog.Run() exits, so there's no lost-message concern.
			ctx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
			_ = applyCfg(ctx)
			cancel()
		}
		prog.Send(msg)
	}()
	go pollUpdate(shutdownCtx, prog, version, p.MihomoBinary())
	go streamTraffic(shutdownCtx, prog, client)
	go pollVersion(shutdownCtx, prog, client)
	go pollProxies(shutdownCtx, prog, client)
	go streamConnections(shutdownCtx, prog, client)
	go pollRules(shutdownCtx, prog, client)
	go streamLogs(shutdownCtx, prog, svc)
	go pollServiceStatus(shutdownCtx, prog, svc)

	_, err = prog.Run()
	shutdownCancel() // wake every goroutine; defer would fire too, but be explicit
	return err
}

func streamTraffic(shutdown context.Context, prog *tea.Program, client *api.Client) {
	for {
		if shutdown.Err() != nil {
			return
		}
		ctx, cancel := context.WithCancel(shutdown)
		ch, errCh := client.Traffic(ctx)
	loop:
		for {
			select {
			case t, ok := <-ch:
				if !ok {
					break loop
				}
				prog.Send(TrafficMsg(t))
			case <-errCh:
				break loop
			case <-shutdown.Done():
				cancel()
				return
			}
		}
		cancel()
		select {
		case <-time.After(2 * time.Second):
		case <-shutdown.Done():
			return
		}
	}
}

func pollProxies(shutdown context.Context, prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(shutdown, 5*time.Second)
		proxies, err := client.GetProxies(ctx)
		cancel()
		if err == nil {
			groups := map[string]msg.ProxyGroup{}
			for name, info := range proxies {
				groups[name] = msg.ProxyGroup{Name: name, Type: info.Type, Now: info.Now, All: info.All}
			}
			prog.Send(msg.ProxiesSnapshot{Groups: groups})
		}
		select {
		case <-ticker.C:
		case <-shutdown.Done():
			return
		}
	}
}

func pollVersion(shutdown context.Context, prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(shutdown, 5*time.Second)
		v, err := client.Version(ctx)
		cancel()
		prog.Send(VersionMsg{Version: v.Version, Err: err})
		select {
		case <-ticker.C:
		case <-shutdown.Done():
			return
		}
	}
}

// pollServiceStatus periodically probes the service manager (systemd-user or
// pid) and pushes the result to the dashboard. Without this loop the
// Dashboard's "Status:" line is stuck at the zero value "○ stopped" because
// no other code path sends msg.ServiceStatus (Bug G).
func pollServiceStatus(shutdown context.Context, prog *tea.Program, svc service.Manager) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(shutdown, 2*time.Second)
		st, err := svc.Status(ctx)
		cancel()
		if err == nil {
			prog.Send(msg.ServiceStatus{
				Running: st.Running,
				PID:     st.PID,
				Mode:    string(st.Mode),
			})
		}
		select {
		case <-ticker.C:
		case <-shutdown.Done():
			return
		}
	}
}

func streamConnections(shutdown context.Context, prog *tea.Program, client *api.Client) {
	for {
		if shutdown.Err() != nil {
			return
		}
		ctx, cancel := context.WithCancel(shutdown)
		ch, errCh := client.Connections(ctx)
	loop:
		for {
			select {
			case snap, ok := <-ch:
				if !ok {
					break loop
				}
				items := make([]msg.ConnectionItem, 0, len(snap.Connections))
				for _, c := range snap.Connections {
					items = append(items, msg.ConnectionItem{
						ID: c.ID, Host: c.Host, Port: c.Port, Network: c.Network,
						Rule: c.Rule, Chains: c.Chains, Upload: c.Upload, Download: c.Download,
					})
				}
				prog.Send(msg.ConnectionsSnapshot{
					DownloadTotal: snap.DownloadTotal,
					UploadTotal:   snap.UploadTotal,
					Items:         items,
				})
			case <-errCh:
				break loop
			case <-shutdown.Done():
				cancel()
				return
			}
		}
		cancel()
		select {
		case <-time.After(2 * time.Second):
		case <-shutdown.Done():
			return
		}
	}
}

func pollRules(shutdown context.Context, prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(shutdown, 5*time.Second)
		rs, errR := client.GetRules(ctx)
		ps, errP := client.GetRuleProviders(ctx)
		cancel()
		if errR == nil && errP == nil {
			snap := msg.RulesSnapshot{}
			for _, r := range rs {
				snap.Rules = append(snap.Rules, msg.RuleEntry{Type: r.Type, Payload: r.Payload, Proxy: r.Proxy})
			}
			for _, p := range ps {
				snap.Providers = append(snap.Providers, msg.RuleProviderEntry{
					Name: p.Name, Behavior: p.Behavior, RuleCount: p.RuleCount, UpdatedAt: p.UpdatedAt,
				})
			}
			prog.Send(snap)
		}
		select {
		case <-ticker.C:
		case <-shutdown.Done():
			return
		}
	}
}

func streamLogs(shutdown context.Context, prog *tea.Program, svc service.Manager) {
	for {
		if shutdown.Err() != nil {
			return
		}
		// Derive from shutdown so canceling propagates to the journalctl/
		// tail subprocess via exec.CommandContext — that's how we get the
		// log-streaming process to actually exit when the TUI quits.
		ctx, cancel := context.WithCancel(shutdown)
		reader, err := svc.Logs(ctx, true)
		if err != nil {
			cancel()
			select {
			case <-time.After(5 * time.Second):
			case <-shutdown.Done():
				return
			}
			continue
		}
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 4096), 1<<20)
		for scanner.Scan() {
			prog.Send(msg.LogLine{Text: scanner.Text()})
		}
		reader.Close()
		cancel()
		select {
		case <-time.After(2 * time.Second):
		case <-shutdown.Done():
			return
		}
	}
}

// ensureConfigSecurity force-rewrites the security-owned keys (ports, auth,
// bind-address, allow-lan, secret) in mihomo's config.yaml so the store stays
// the single source of truth. No-op if the file does not exist (bootstrap
// will create it). Returns true if the file was modified.
func ensureConfigSecurity(st *store.Store, configFile string) (bool, error) {
	if _, err := os.Stat(configFile); err != nil {
		return false, nil
	}
	return config.EnsureSecurityFields(configFile, config.SecurityFields{
		MixedPort:        st.Cfg.MixedPort,
		ControllerPort:   st.Cfg.ControllerPort,
		ControllerSecret: st.Cfg.ControllerSecret,
		ProxyUser:        st.Cfg.ProxyUser,
		ProxyPass:        st.Cfg.ProxyPass,
	})
}

// reconcilePorts picks free TCP ports for mixed-port and external-controller.
// If the saved ports are busy and mihomo is not running, scans upward and
// persists. Deletes any pre-existing config.yaml that referenced the stale
// ports so bootstrap re-emits a matching one.
func reconcilePorts(svc service.Manager, st *store.Store, configFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if status, err := svc.Status(ctx); err == nil && status.Running {
		return nil
	}
	mp, err := portutil.FindFree(st.Cfg.MixedPort, 100)
	if err != nil {
		return fmt.Errorf("mixed-port: %w", err)
	}
	cp, err := portutil.FindFree(st.Cfg.ControllerPort, 100)
	if err != nil {
		return fmt.Errorf("controller-port: %w", err)
	}
	// Configs whose mixed and controller starts fall within 100 of each other
	// could have both scans converge on the same port. Push the controller
	// past the chosen mixed-port if so.
	if mp == cp {
		alt, err := portutil.FindFree(cp+1, 100)
		if err != nil {
			return fmt.Errorf("controller-port collision: %w", err)
		}
		cp = alt
	}
	if mp == st.Cfg.MixedPort && cp == st.Cfg.ControllerPort {
		return nil
	}
	st.Cfg.MixedPort = mp
	st.Cfg.ControllerPort = cp
	if err := st.Save(); err != nil {
		return err
	}
	// Force bootstrap to regenerate the mihomo config so it reflects new ports.
	_ = os.Remove(configFile)
	return nil
}

