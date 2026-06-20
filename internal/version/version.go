package version

import "strings"

// Version is the app version shown in the TUI. Keep this aligned with release tags.
var Version = "1.0.2"

func Display() string {
	value := strings.TrimSpace(Version)
	if value == "" {
		return "v0.0.0"
	}
	if strings.HasPrefix(value, "v") {
		return value
	}
	return "v" + value
}
