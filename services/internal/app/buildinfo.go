// Package app holds process-wide build metadata shared by the api, worker and mcctl binaries.
package app

import "runtime/debug"

// Version and Commit are set at build time with -ldflags "-X …".
var (
	Version = "0.1.0-dev"
	Commit  = ""
)

// BuildVersion returns the version string reported by /api/v1/config and telemetry.
func BuildVersion() string {
	commit := Commit
	if commit == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && len(s.Value) >= 7 {
					commit = s.Value[:7]
				}
			}
		}
	}
	if commit == "" {
		return Version
	}
	return Version + "+" + commit
}
