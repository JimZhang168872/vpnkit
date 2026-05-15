package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"vpnkit/internal/api"
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

// toProfilesProfiles converts store.Profile slice to profiles.Profile slice.
func toProfilesProfiles(in []store.Profile) []profiles.Profile {
	out := make([]profiles.Profile, len(in))
	for i, x := range in {
		out[i] = profiles.Profile{Name: x.Name, URL: x.URL, UserAgent: x.UserAgent, LastUpdated: x.LastUpdated}
	}
	return out
}
