// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime/debug"
	"sync"
)

// resolveBuild returns the version and commit to report. Release builds stamp
// both with ldflags. A `go install ...@v0.8.1` build does not, so a stamp that
// is empty or "dev" falls back to the module version and the vcs settings the
// Go toolchain records. A "(devel)" module version stays "dev".
func resolveBuild(version, commit string, info *debug.BuildInfo) (string, string) {
	if version != "" && version != "dev" {
		return version, commit
	}
	if version == "" {
		version = "dev"
	}
	if info == nil {
		return version, commit
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		version = v
	}
	if commit == "" {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				commit = s.Value
			}
		}
	}
	return version, commit
}

// currentBuild resolves the running binary's version and commit.
func currentBuild() (string, string) {
	info, _ := debug.ReadBuildInfo()
	return resolveBuild(buildVersion, buildCommit, info)
}

// telemetryVersion is the version hook telemetry records, the same one
// `downshift version` and doctor show. It is resolved once per process.
var telemetryVersion = sync.OnceValue(func() string {
	v, _ := currentBuild()
	return v
})
