package buildinfo

import (
	"fmt"
)

var (
	BuildInfoVersion = "local"
	BuildInfoCommit  = "unknown"
	BuildInfoDate    = "unknown"
)

func BuildInfo() string {
	return fmt.Sprintf(
		"Version: %s (gitsha1: %s) - built: %s\n",
		BuildInfoVersion,
		BuildInfoCommit,
		BuildInfoDate,
	)
}
