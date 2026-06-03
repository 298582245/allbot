package version

import "testing"

func TestDisplayVersion(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v2.0.0"
	if got := DisplayVersion(); got != "AllBot v2.0.0" {
		t.Fatalf("DisplayVersion = %q", got)
	}

	Version = ""
	if got := DisplayVersion(); got != "AllBot unknown" {
		t.Fatalf("DisplayVersion empty = %q", got)
	}
}
