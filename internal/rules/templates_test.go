package rules

import (
	"strings"
	"testing"
)

func TestLoadKnownTemplates(t *testing.T) {
	tests := []struct {
		name     string
		contains string
	}{
		{"loyalsoldier", "rule-providers"},
		{"minimal", "GEOIP,CN,🎯 Direct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Load(tt.name)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !strings.Contains(string(data), tt.contains) {
				t.Errorf("template missing %q: %s", tt.contains, string(data))
			}
		})
	}
}

func TestLoadUnknown(t *testing.T) {
	if _, err := Load("nope"); err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestList(t *testing.T) {
	got := List()
	want := map[string]bool{"loyalsoldier": false, "minimal": false}
	for _, n := range got {
		want[n] = true
	}
	for k, v := range want {
		if !v {
			t.Errorf("missing template in List(): %s", k)
		}
	}
}
