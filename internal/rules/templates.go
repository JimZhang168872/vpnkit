// Package rules manages embedded rule-set templates that get merged into mihomo's config.
package rules

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed templates/*.yaml
var tmplFS embed.FS

// Load returns the raw YAML bytes of a named template.
func Load(name string) ([]byte, error) {
	data, err := tmplFS.ReadFile("templates/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("rules: unknown template %q", name)
	}
	return data, nil
}

// List enumerates available template names (without extension), sorted alphabetically.
func List() []string {
	var out []string
	_ = fs.WalkDir(tmplFS, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.TrimSuffix(d.Name(), ".yaml")
		out = append(out, name)
		return nil
	})
	sort.Strings(out)
	return out
}
