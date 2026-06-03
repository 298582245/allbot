package version

import "strings"

var Version = "v1.0.0"
var Commit = "unknown"
var BuildTime = "unknown"

func DisplayVersion() string {
	current := strings.TrimSpace(Version)
	if current == "" {
		current = "unknown"
	}
	return "AllBot " + current
}
