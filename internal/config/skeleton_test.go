package config

import (
	"strings"
	"testing"
)

func TestBuildSkeletonIncludesController(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "deadbeef",
		LogLevel:         "info",
		RuleTemplate:     "minimal",
	})
	if err != nil {
		t.Fatalf("BuildSkeleton: %v", err)
	}
	s := string(yaml)
	mustContain(t, s, "mixed-port: 7890")
	mustContain(t, s, "external-controller: 127.0.0.1:9090")
	mustContain(t, s, "secret: deadbeef")
	mustContain(t, s, "GEOIP,CN,🎯 Direct")
}

func TestBuildSkeletonIncludesDefaultGroups(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort:        7890,
		ControllerPort:   9090,
		ControllerSecret: "x",
		RuleTemplate:     "minimal",
	})
	if err != nil {
		t.Fatalf("BuildSkeleton: %v", err)
	}
	s := string(yaml)
	mustContain(t, s, "🚀 Proxy")
	mustContain(t, s, "🎯 Direct")
	mustContain(t, s, "🛑 Reject")
}

func TestBuildSkeletonUnknownTemplate(t *testing.T) {
	_, err := BuildSkeleton(SkeletonInput{RuleTemplate: "nope"})
	if err == nil {
		t.Error("expected error for unknown template")
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing %q in:\n%s", needle, haystack)
	}
}
