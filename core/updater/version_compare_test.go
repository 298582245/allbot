package updater

import "testing"

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int
	}{
		{name: "older", current: "v1.0.0", latest: "v1.0.1", want: -1},
		{name: "same without v", current: "v1.0.1", latest: "1.0.1", want: 0},
		{name: "newer", current: "v1.2.0", latest: "v1.1.9", want: 1},
		{name: "major older", current: "v1.9.9", latest: "v2.0.0", want: -1},
		{name: "uppercase v", current: "V1.0.0", latest: "v1.0.0", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersion(tt.current, tt.latest)
			if err != nil {
				t.Fatalf("CompareVersion returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CompareVersion = %d, expected %d", got, tt.want)
			}
		})
	}
}

func TestCompareVersionInvalid(t *testing.T) {
	tests := []struct {
		current string
		latest  string
	}{
		{current: "", latest: "v1.0.0"},
		{current: "v1.0", latest: "v1.0.0"},
		{current: "v1.0.x", latest: "v1.0.0"},
		{current: "v1.0.0-beta", latest: "v1.0.0"},
		{current: "v1.0.0", latest: "v1.0.0+build"},
	}
	for _, tt := range tests {
		if _, err := CompareVersion(tt.current, tt.latest); err == nil {
			t.Fatalf("CompareVersion(%q, %q) expected error", tt.current, tt.latest)
		}
	}
}
