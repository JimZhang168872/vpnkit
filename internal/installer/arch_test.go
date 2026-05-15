package installer

import "testing"

func TestAssetNameAmd64Compatible(t *testing.T) {
	got := assetName("amd64", true, "v1.19.16")
	want := "mihomo-linux-amd64-compatible-v1.19.16.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestAssetNameAmd64Modern(t *testing.T) {
	got := assetName("amd64", false, "v1.19.16")
	want := "mihomo-linux-amd64-v1.19.16.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestAssetNameArm64(t *testing.T) {
	got := assetName("arm64", false, "v1.19.16")
	want := "mihomo-linux-arm64-v1.19.16.gz"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestNeedsCompatibleParsesCpuinfo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"modern flags present", "flags : fpu vme popcnt sse4_2 avx2\n", false},
		{"missing popcnt", "flags : fpu vme sse4_2 avx2\n", true},
		{"missing sse4_2", "flags : fpu vme popcnt avx2\n", true},
		{"empty input", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsCompatibleFromCpuinfo(tt.input); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}
