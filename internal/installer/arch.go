// Package installer downloads, verifies, and unpacks mihomo release binaries.
package installer

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// SelectAsset returns (assetName, useCompatible) for the current host.
// On non-amd64 architectures `useCompatible` is always false.
func SelectAsset(version string) (string, bool) {
	arch := runtime.GOARCH
	compat := false
	if arch == "amd64" {
		compat = NeedsCompatibleBuild()
	}
	return assetName(arch, compat, version), compat
}

// NeedsCompatibleBuild reports whether the running CPU lacks features the modern
// (non-compatible) mihomo build assumes. Reads /proc/cpuinfo; returns true on error
// so we err on the side of compatibility.
func NeedsCompatibleBuild() bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return true
	}
	return needsCompatibleFromCpuinfo(string(data))
}

func needsCompatibleFromCpuinfo(s string) bool {
	flagsLine := ""
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "flags") {
			flagsLine = line
			break
		}
	}
	if flagsLine == "" {
		return true
	}
	flags := strings.Fields(flagsLine)
	have := map[string]bool{}
	for _, f := range flags {
		have[f] = true
	}
	required := []string{"popcnt", "sse4_2"}
	for _, r := range required {
		if !have[r] {
			return true
		}
	}
	return false
}

func assetName(arch string, compatible bool, version string) string {
	suffix := ""
	if compatible {
		suffix = "-compatible"
	}
	return fmt.Sprintf("mihomo-linux-%s%s-%s.gz", arch, suffix, version)
}
