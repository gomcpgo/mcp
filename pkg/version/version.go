package version

import (
	"fmt"
	"runtime"
)

// Version is the current version of the MCP library.
// This is manually updated with each release.
const Version = "v0.1.1"

// Info contains versioning information.
type Info struct {
	Version   string `json:"version"`
	GoVersion string `json:"goVersion,omitempty"`
}

// GetInfo returns versioning information.
func GetInfo() Info {
	return Info{
		Version:   Version,
		GoVersion: runtime.Version(),
	}
}

// String returns the string representation of versioning information.
func (i Info) String() string {
	return fmt.Sprintf("Version=%s GoVersion=%s", i.Version, i.GoVersion)
}
