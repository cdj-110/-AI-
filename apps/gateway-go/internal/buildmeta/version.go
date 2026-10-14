package buildmeta

import "strings"

// These values are intentionally variables so release builds can override
// them with -ldflags without changing application code.
var (
	Version   = "V0.7"
	BuildTime = ""
	Commit    = ""
)

func DisplayVersion() string {
	value := strings.TrimSpace(Version)
	if value == "" {
		return "V0.7"
	}
	if strings.HasPrefix(strings.ToUpper(value), "V") {
		return value
	}
	return "V" + value
}
