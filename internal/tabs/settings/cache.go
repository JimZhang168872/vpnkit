package settings

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"vpnkit/internal/paths"
)

type cacheModel struct {
	dir  string
	last int64
}

func newCache(p paths.XDG) cacheModel {
	m := cacheModel{dir: p.VpnkitCache}
	m.last = m.Size()
	return m
}

func (m cacheModel) Size() int64 {
	if m.dir == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(m.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (m cacheModel) Clear() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(m.dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (m cacheModel) Update(message tea.Msg) (cacheModel, tea.Cmd) {
	if km, ok := message.(tea.KeyMsg); ok && km.String() == "c" {
		_ = m.Clear()
		m.last = m.Size()
	}
	return m, nil
}

func (m cacheModel) View(width, height int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Cache")
	body := header + "\n\n" +
		fmt.Sprintf("  Path : %s\n", m.dir) +
		fmt.Sprintf("  Size : %s\n", human(m.last)) +
		"\n  [c] clear cache\n"
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 2).Render(body)
}

func human(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)
	switch {
	case n >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(GiB))
	case n >= MiB:
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
