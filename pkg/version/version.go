package version

import "runtime/debug"

// Version is set via ldflags during build
var Version string

func GetVersion() string {
	// If version was set via ldflags, use it
	if Version != "" {
		return Version
	}

	// Fall back to build info
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	version := info.Main.Version
	if version == "" || version == "(devel)" {
		return "dev"
	}
	return version
}
