package version

import "testing"

func TestDisplayVersion(t *testing.T) {
	oldVersion := Version
	oldChannel := BuildChannel
	defer func() {
		Version = oldVersion
		BuildChannel = oldChannel
	}()

	Version = "v2.0.0"
	BuildChannel = "release"
	if got := DisplayVersion(); got != "AllBot v2.0.0" {
		t.Fatalf("DisplayVersion = %q", got)
	}

	Version = ""
	if got := DisplayVersion(); got != "AllBot unknown" {
		t.Fatalf("DisplayVersion empty = %q", got)
	}
}

func TestDisplayVersionShowsLocalChannel(t *testing.T) {
	oldVersion := Version
	oldChannel := BuildChannel
	Version = "v1.0.1"
	BuildChannel = "local"
	defer func() {
		Version = oldVersion
		BuildChannel = oldChannel
	}()

	if got := DisplayVersion(); got != "AllBot v1.0.1 (local)" {
		t.Fatalf("DisplayVersion = %q", got)
	}
}
