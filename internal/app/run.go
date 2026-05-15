package app

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
	"vpnkit/internal/msg"
	"vpnkit/internal/paths"
	"vpnkit/internal/profiles"
	"vpnkit/internal/service"
	"vpnkit/internal/store"
)

// Run launches the vpnkit TUI. Returns the bubbletea exit error.
func Run() error {
	p := paths.Resolve()
	if err := p.Ensure(); err != nil {
		return fmt.Errorf("paths: %w", err)
	}
	st, err := store.Load(p.VpnkitConfigFile())
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	// Detect service mode on first run.
	if st.Cfg.ServiceMode == "" {
		mode := service.Detect(nil)
		st.Cfg.ServiceMode = string(mode)
		_ = st.Save()
	}
	svc := service.New(service.Mode(st.Cfg.ServiceMode), service.Config{
		BinaryPath:  p.MihomoBinary(),
		ConfigDir:   p.MihomoConfig,
		PIDFilePath: p.PIDFile(),
		LogFilePath: p.MihomoLog(),
		UnitPath:    p.SystemdUnit(),
	})
	client := api.New(fmt.Sprintf("http://127.0.0.1:%d", st.Cfg.ControllerPort), st.Cfg.ControllerSecret)

	profMgr := profiles.New(profiles.Config{
		ConfigYAMLPath:   p.MihomoConfigFile(),
		PatchPath:        filepath.Join(p.MihomoConfig, "patch.yaml"),
		ControllerPort:   st.Cfg.ControllerPort,
		ControllerSecret: st.Cfg.ControllerSecret,
		RuleTemplate:     st.Cfg.RuleTemplate,
	})
	profMgr.Load(toProfilesProfiles(st.Cfg.Profiles), st.Cfg.ActiveProfile)
	profMgr.SetOnChange(func() {
		persisted := make([]store.Profile, 0)
		for _, p := range profMgr.All() {
			persisted = append(persisted, store.Profile{
				Name:        p.Name,
				URL:         p.URL,
				UserAgent:   p.UserAgent,
				LastUpdated: p.LastUpdated,
			})
		}
		st.Cfg.Profiles = persisted
		st.Cfg.ActiveProfile = profMgr.Active()
		_ = st.Save()
	})

	model := NewModel(client, profMgr)
	prog := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		msg := MaybeBootstrap(BootstrapDeps{
			Paths:   p,
			Store:   st,
			Service: svc,
		})()
		prog.Send(msg)
	}()
	go streamTraffic(prog, client)
	go pollVersion(prog, client)
	go pollProxies(prog, client)
	go streamConnections(prog, client)
	go pollRules(prog, client)
	go streamLogs(prog, svc)

	_, err = prog.Run()
	return err
}

func streamTraffic(prog *tea.Program, client *api.Client) {
	for {
		ctx, cancel := context.WithCancel(context.Background())
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
			}
		}
		cancel()
		time.Sleep(2 * time.Second) // backoff before reconnect
	}
}

func pollProxies(prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		proxies, err := client.GetProxies(ctx)
		cancel()
		if err == nil {
			groups := map[string]msg.ProxyGroup{}
			for name, info := range proxies {
				groups[name] = msg.ProxyGroup{Name: name, Type: info.Type, Now: info.Now, All: info.All}
			}
			prog.Send(msg.ProxiesSnapshot{Groups: groups})
		}
		<-ticker.C
	}
}

func pollVersion(prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		v, err := client.Version(ctx)
		cancel()
		prog.Send(VersionMsg{Version: v.Version, Err: err})
		<-ticker.C
	}
}

func streamConnections(prog *tea.Program, client *api.Client) {
	for {
		ctx, cancel := context.WithCancel(context.Background())
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
			}
		}
		cancel()
		time.Sleep(2 * time.Second)
	}
}

func pollRules(prog *tea.Program, client *api.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		<-ticker.C
	}
}

func streamLogs(prog *tea.Program, svc service.Manager) {
	for {
		ctx, cancel := context.WithCancel(context.Background())
		reader, err := svc.Logs(ctx, true)
		if err != nil {
			cancel()
			time.Sleep(5 * time.Second)
			continue
		}
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 4096), 1<<20)
		for scanner.Scan() {
			prog.Send(msg.LogLine{Text: scanner.Text()})
		}
		reader.Close()
		cancel()
		time.Sleep(2 * time.Second)
	}
}

// toProfilesProfiles converts store.Profile slice to profiles.Profile slice.
func toProfilesProfiles(in []store.Profile) []profiles.Profile {
	out := make([]profiles.Profile, len(in))
	for i, x := range in {
		out[i] = profiles.Profile{Name: x.Name, URL: x.URL, UserAgent: x.UserAgent, LastUpdated: x.LastUpdated}
	}
	return out
}
