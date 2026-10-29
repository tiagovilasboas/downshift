// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// binaryCopy is another downshift executable a shell may resolve. built is the
// build stamp it reports; empty when it reports none (an older build).
type binaryCopy struct {
	path  string
	built string
}

// binaryProbeTimeout bounds one --version call on a candidate. A copy that does
// not answer in time is not listed.
const binaryProbeTimeout = 3 * time.Second

// buildLine matches the version line main prints: "downshift <version> (commit
// <c>, built <stamp>)". The stamp is group 1.
var buildLine = regexp.MustCompile(`^downshift \S.*\(commit \S+, built ([^)]*)\)`)

// defaultBinDirs lists the directories a shell may resolve downshift from: the
// PATH entries, the Go install directories, and the per-user bin directory a
// manual copy uses. Each directory appears once.
func defaultBinDirs(home string) []string {
	dirs := append(filepath.SplitList(os.Getenv("PATH")),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".local", "bin"))
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		dirs = append(dirs, gobin)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		dirs = append(dirs, filepath.Join(gopath, "bin"))
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		d = filepath.Clean(d)
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// binaryName is the executable name on this OS.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "downshift.exe"
	}
	return "downshift"
}

// findBinaryCopies returns the downshift executables in dirs, other than self.
// Only regular files count, so symbolic links are skipped: removing one would
// free nothing, and its target is listed on its own when it lies in a listed
// directory. A file counts only when it answers --version as downshift, so an
// unrelated file with the same name is never listed.
func findBinaryCopies(dirs []string, self string) []binaryCopy {
	name := binaryName()
	selfReal := resolvePath(self)
	var out []binaryCopy
	seen := map[string]bool{}
	for _, d := range dirs {
		p := filepath.Join(d, name)
		fi, err := os.Lstat(p)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		real := resolvePath(p)
		if real == selfReal || seen[real] {
			continue
		}
		seen[real] = true
		if built, ok := probeBuild(p); ok {
			out = append(out, binaryCopy{path: p, built: built})
		}
	}
	return out
}

// probeBuild runs an executable with --version. ok is false unless the first
// line is downshift's own version line; built is the stamp when it has one.
func probeBuild(path string) (built string, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), binaryProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if !strings.HasPrefix(line, "downshift ") {
		return "", false
	}
	if m := buildLine.FindStringSubmatch(line); m != nil {
		return m[1], true
	}
	return "", true
}
