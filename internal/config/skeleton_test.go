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

func TestBuildSkeletonGeoxUrlPointsAtGitHubDirectly(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "x", RuleTemplate: "minimal",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(yaml)
	if !strings.Contains(s, "geox-url") {
		t.Errorf("geox-url should always be present:\n%s", s)
	}
	if !strings.Contains(s, "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb") {
		t.Errorf("expected direct GitHub geoip URL:\n%s", s)
	}
	if strings.Contains(s, "jsdelivr") || strings.Contains(s, "ghproxy") {
		t.Errorf("geox-url should not reference any mirror:\n%s", s)
	}
}

func TestBuildSkeletonEmitsAuthentication(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "x",
		RuleTemplate: "minimal",
		ProxyUser:    "alice",
		ProxyPass:    "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(yaml), "authentication:")
	mustContain(t, string(yaml), "alice:s3cret")
}

func TestBuildSkeletonOmitsAuthenticationWhenMissingCreds(t *testing.T) {
	yaml, err := BuildSkeleton(SkeletonInput{
		MixedPort: 7890, ControllerPort: 9090, ControllerSecret: "x",
		RuleTemplate: "minimal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(yaml), "authentication:") {
		t.Errorf("authentication: should be absent when creds unset:\n%s", string(yaml))
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("missing %q in:\n%s", needle, haystack)
	}
}
