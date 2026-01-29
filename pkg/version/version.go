package version

import "runtime/debug"

func GetVersion() string {
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
