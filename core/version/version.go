package version

import "strings"

var Version = "v1.0.1"
var Commit = "unknown"
var BuildTime = "unknown"
var BuildChannel = "local"

func DisplayVersion() string {
	current := strings.TrimSpace(Version)
	if current == "" {
		current = "unknown"
	}
	channel := NormalizedBuildChannel()
	if channel == "" || channel == "release" {
		return "AllBot " + current
	}
	return "AllBot " + current + " (" + channel + ")"
}

func NormalizedBuildChannel() string {
	return strings.ToLower(strings.TrimSpace(BuildChannel))
}
