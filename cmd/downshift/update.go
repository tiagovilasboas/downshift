// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/selfupdate"
)

// `downshift update [--check] [--yes] [--version vX.Y.Z]` installs the latest
// release over this binary, then runs the NEW binary's setup (which shows the
// diff and asks before writing, unless --yes) and doctor. The binary step is
// skipped when this build is already current; setup still runs, because a
// harness file can lag behind the binary.

// releaseSource is what update needs from GitHub; tests pass a fake.
type releaseSource interface {
	LatestTag() (string, error)
	Download(tag, asset string) ([]byte, error)
}

type updateDeps struct {
	Source  releaseSource
	Exe     string
	Current string
	GOOS    string
	GOARCH  string
	// Attest checks build provenance of the archive; nil skips it.
	Attest func(archive []byte, name string) (verified bool, note string)
	// Run runs the installed binary with args, wired to the terminal.
	Run func(bin string, args ...string) int
}

type updateFlags struct {
	check, yes bool
	version    string
}

func runUpdate(args []string, in io.Reader, out, errw io.Writer) int {
	flags, code := parseUpdateArgs(args, out, errw)
	if code >= 0 {
		return code
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errw, "update: cannot find this binary: %v\n", err)
		return 1
	}
	deps := updateDeps{
		Source: selfupdate.NewSource(), Exe: exe, Current: buildVersion,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		Attest: ghAttest, Run: terminalRunner(in, out, errw),
	}
	return update(flags, deps, out, errw)
}

func parseUpdateArgs(args []string, out, errw io.Writer) (updateFlags, int) {
	var f updateFlags
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprint(out, updateHelp["update"])
			return f, 0
		case a == "--check":
			f.check = true
		case a == "--yes" || a == "-y":
			f.yes = true
		case a == "--version" && i+1 < len(args):
			i++
			f.version = args[i]
		case strings.HasPrefix(a, "--version="):
			f.version = strings.TrimPrefix(a, "--version=")
		default:
			fmt.Fprintf(errw, "update: unknown argument %q\n%s", a, updateHelp["update"])
			return f, 2
		}
	}
	if f.version != "" && !strings.HasPrefix(f.version, "v") {
		f.version = "v" + f.version
	}
	return f, -1
}

func update(f updateFlags, d updateDeps, out, errw io.Writer) int {
	tag := f.version
	if tag == "" {
		latest, err := d.Source.LatestTag()
		if err != nil {
			fmt.Fprintf(errw, "update: %v\n", err)
			return 1
		}
		tag = latest
	}
	needed := binaryUpdateNeeded(d.Current, tag, f.version != "")
	if f.check {
		reportCheck(out, d.Current, tag, needed)
		return 0
	}
	bin := d.Exe
	if needed {
		installed, code := installRelease(tag, d, out, errw)
		if code != 0 {
			return code
		}
		bin = installed
	} else {
		fmt.Fprintf(out, "downshift %s is current (latest release %s); checking the harness files.\n", d.Current, tag)
	}
	return refreshHarnesses(bin, f.yes, d, out)
}

// binaryUpdateNeeded is true when tag is newer than current, when current is
// not a release version (a dev build), or when the user named a version.
func binaryUpdateNeeded(current, tag string, pinned bool) bool {
	if pinned {
		return true
	}
	c, ok := compareVersions(current, tag)
	return !ok || c < 0
}

func reportCheck(out io.Writer, current, tag string, needed bool) {
	if needed {
		fmt.Fprintf(out, "Update available: %s -> %s. Run: downshift update\n", current, tag)
		return
	}
	fmt.Fprintf(out, "downshift %s is current (latest release %s).\n", current, tag)
}

// refreshHarnesses runs setup and doctor from the binary just installed, so
// the harness files come from the new release, not from this process.
func refreshHarnesses(bin string, yes bool, d updateDeps, out io.Writer) int {
	setupArgs := []string{"setup"}
	if yes {
		setupArgs = append(setupArgs, "--yes")
	}
	fmt.Fprintln(out, "\nUpdating harness files (setup shows the diff and asks before writing):")
	if code := d.Run(bin, setupArgs...); code != 0 {
		return code
	}
	fmt.Fprintln(out, "\nChecking the result:")
	return d.Run(bin, "doctor")
}

func terminalRunner(in io.Reader, out, errw io.Writer) func(string, ...string) int {
	return func(bin string, args ...string) int {
		cmd := exec.Command(bin, args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errw
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode()
			}
			fmt.Fprintf(errw, "update: running %s %s: %v\n", bin, strings.Join(args, " "), err)
			return 1
		}
		return 0
	}
}
