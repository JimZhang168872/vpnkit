// Package patch loads a user-edited overlay YAML and deep-merges it into a mihomo
// config map. Maps merge recursively; arrays in the patch replace target arrays.
package patch

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Apply reads patchPath (no-op if missing) and merges its contents into target.
func Apply(patchPath string, target map[string]any) error {
	data, err := os.ReadFile(patchPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var overlay map[string]any
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return err
	}
	deepMerge(target, overlay)
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if existingMap, ok := dst[k].(map[string]any); ok {
			if newMap, ok := v.(map[string]any); ok {
				deepMerge(existingMap, newMap)
				continue
			}
		}
		dst[k] = v
	}
}
