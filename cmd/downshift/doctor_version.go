// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// The version-sync check for `doctor`. A binary left at an old release while
// the plugin and the OpenCode bridge moved on (or the reverse) used to show a
// green "version" row. This check reads the stamps on disk, offline:
//
//   - the Claude Code plugin's recorded version, from its install registry;
//   - the OpenCode bridge, which carries no number but is a byte copy of what
//     this binary embeds, so a different copy is a stamp from another release;
//   - the other downshift binaries on PATH, run with `version`;
//   - the latest release, only when the caller injected env.Latest
//     (`doctor --check-latest`). Unknown never fails.

const versionSyncID = "version sync"

const versionRunTimeout = 2 * time.Second

// maxPathCopiesRun caps how many PATH copies doctor runs, so a long PATH
// cannot stretch doctor past a few seconds.
const maxPathCopiesRun = 4

// checkVersionSync adds the version rows. binary is the running binary's version.
func checkVersionSync(rep *doctorReport, env doctorEnv, binary string) {
	checkStampSync(rep, env, binary)
	checkNewestOnPath(rep, env, binary)
	if env.Latest != nil {
		checkLatestRelease(rep, env.Latest, binary)
	}
}

func checkStampSync(rep *doctorReport, env doctorEnv, binary string) {
	var bad, good []string
	// A pseudo-version (`go install …@main`) or pre-release build sits
	// between releases, so no plugin stamp can match it: skip the compare.
	if _, ok := parseSemver(binary); ok && !strings.Contains(binary, "-") {
		if v, found := pluginStamp(env.Home); found {
			if cmp, ok := compareVersions(v, binary); ok {
				if cmp != 0 {
					bad = append(bad, fmt.Sprintf("the Claude Code plugin is at %s", v))
				} else {
					good = append(good, "plugin")
				}
			}
		}
	}
	if stale, found := bridgeStale(env.Home); found {
		if stale {
			bad = append(bad, "the OpenCode bridge is a copy from another release")
		} else {
			good = append(good, "OpenCode bridge")
		}
	}
	switch {
	case len(bad) > 0:
		rep.add(doctorCheck{ID: versionSyncID, Status: doctorFail,
			Detail: fmt.Sprintf("this binary is %s but %s", binary, strings.Join(bad, " and ")),
			Fix:    "downshift setup --yes (refreshes the bridge and the plugin), or reinstall the binary so all match"})
	case len(good) > 0:
		rep.add(doctorCheck{ID: versionSyncID, Status: doctorOK,
			Detail: fmt.Sprintf("%s match this binary (%s)", strings.Join(good, " and "), binary)})
	}
}

// pluginStamp returns the version the Claude Code plugin registry records for
// the first user install that carries one.
func pluginStamp(home string) (string, bool) {
	t, ok := pluginTarget("claude-code")
	if !ok {
		return "", false
	}
	reg := filepath.Join(home, filepath.FromSlash(t.ConfigDir), filepath.FromSlash(setup.PluginRegistryRel))
	installs, err := readPluginInstalls(reg, t.Plugin)
	if err != nil {
		return "", false
	}
	for _, in := range installs {
		if _, ok := parseSemver(in.Version); ok {
			return in.Version, true
		}
	}
	return "", false
}

// bridgeStale reports whether an installed bridge differs from the embedded
// copy. found is false when no bridge file of ours is installed.
func bridgeStale(home string) (stale, found bool) {
	t, ok := generatedFilesTarget("opencode")
	if !ok {
		return false, false
	}
	for _, f := range t.Files {
		want, err := fs.ReadFile(f.FS, f.Source)
		if err != nil {
			continue
		}
		cur, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(f.Dest)))
		if err != nil || !configfile.Owns(cur) {
			continue
		}
		found = true
		if string(cur) != string(want) {
			stale = true
		}
	}
	return stale, found
}

// checkNewestOnPath runs every downshift on PATH and warns when the one that
// runs first is not the newest. Without a second copy it reports nothing.
func checkNewestOnPath(rep *doctorReport, env doctorEnv, binary string) {
	if env.PathCopies == nil || env.RunVersion == nil {
		return
	}
	// Only absolute PATH entries are run: a relative one (".", "bin") would
	// execute whatever the current directory holds.
	var copies []string
	for _, c := range env.PathCopies() {
		if filepath.IsAbs(c) && len(copies) < maxPathCopiesRun {
			copies = append(copies, c)
		}
	}
	if len(copies) < 2 {
		return
	}
	newest, newestVer := copies[0], ""
	firstVer := ""
	for i, c := range copies {
		out, err := env.RunVersion(c)
		v := versionFromOutput(out)
		if err != nil || v == "" {
			continue
		}
		if i == 0 {
			firstVer = v
		}
		if newestVer == "" {
			newest, newestVer = c, v
		} else if cmp, ok := compareVersions(v, newestVer); ok && cmp > 0 {
			newest, newestVer = c, v
		}
	}
	if firstVer == "" || newestVer == "" {
		return
	}
	if cmp, ok := compareVersions(firstVer, newestVer); !ok || cmp >= 0 {
		rep.add(doctorCheck{ID: "newest on PATH", Status: doctorOK,
			Detail: fmt.Sprintf("%s runs first and is the newest of %d copies (%s)", copies[0], len(copies), firstVer)})
		return
	}
	rep.add(doctorCheck{ID: "newest on PATH", Status: doctorWarn,
		Detail: fmt.Sprintf("%s (%s) runs first, but %s is newer (%s)", copies[0], firstVer, newest, newestVer),
		Fix:    fmt.Sprintf("ln -sf %q %q, or remove the older copy", newest, copies[0])})
}

// versionFromOutput reads the version out of `downshift version` output:
// "downshift v0.8.1 (commit ..., built ...)".
func versionFromOutput(out string) string {
	f := strings.Fields(out)
	if len(f) < 2 || f[0] != "downshift" {
		return ""
	}
	// Only a version shape is accepted, so a copy that prints junk cannot put
	// text into the doctor output.
	if _, ok := parseSemver(f[1]); ok || f[1] == devBuildVersion {
		return f[1]
	}
	return ""
}

// runVersionOf runs `<path> version` with a short timeout.
func runVersionOf(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionRunTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "version")
	// A wrapper whose child keeps stdout open would hold Output() past the
	// timeout; WaitDelay bounds the wait after the kill.
	cmd.WaitDelay = 500 * time.Millisecond
	out, err := cmd.Output()
	return string(out), err
}

func checkLatestRelease(rep *doctorReport, latest func() (string, bool), binary string) {
	const id = "latest release"
	tag, ok := latest()
	if !ok {
		rep.add(doctorCheck{ID: id, Status: doctorOK, Detail: "unknown (offline or rate limited); not a failure"})
		return
	}
	cmp, comparable := compareVersions(binary, tag)
	if !comparable || cmp >= 0 {
		rep.add(doctorCheck{ID: id, Status: doctorOK, Detail: fmt.Sprintf("latest is %s, this binary is %s", tag, binary)})
		return
	}
	rep.add(doctorCheck{ID: id, Status: doctorWarn,
		Detail: fmt.Sprintf("%s is available, this binary is %s", tag, binary),
		Fix:    "reinstall (install.sh or `go install github.com/tiagovilasboas/downshift-ai/cmd/downshift@" + tag + "`), then run `downshift setup --yes`"})
}
