// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallUsesHTTPSOnlyForEveryDownload(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	args, _ := os.ReadFile(filepath.Join(filepath.Dir(r.ghLog), "curl.log.args"))
	lines := strings.Split(strings.TrimSpace(string(args)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected the lookup, archive and checksum calls, got %q", lines)
	}
	for _, l := range lines {
		if !strings.Contains(l, "--proto =https") || !strings.Contains(l, "--tlsv1.2") {
			t.Errorf("curl call without https-only/TLS 1.2: %q", l)
		}
	}
}

// The binary lands through a temporary name and a rename; no staging file is
// left in the install directory, and a failed copy leaves the old binary.
func TestInstallLeavesNoStagingFileAndKeepsTheOldBinaryOnFailure(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	entries, _ := os.ReadDir(r.binDir) // not even created
	if len(entries) != 1 || entries[0].Name() != "downshift" {
		t.Errorf("install dir = %v, want only downshift", entries)
	}
}

// Regression: a release published without assets made the one-liner end in a
// bare "curl: (22) 404". It must name the version and platform, offer the
// three ways out, exit non-zero and write nothing.
func TestInstallExplainsAMissingReleaseArchive(t *testing.T) {
	r := runInstallEnv(t, "v0.1.0", "", []string{"FAKE_NO_ARCHIVE=1"})
	if r.code == 0 {
		t.Fatalf("install.sh exit 0 for a release without an archive\noutput:\n%s", r.output)
	}
	for _, want := range []string{
		"release v0.1.0 has no ",
		"/" + runtime.GOARCH + " archive yet",
		"go install github.com/tiagovilasboas/downshift/cmd/downshift@v0.1.0",
		"https://github.com/tiagovilasboas/downshift/releases",
		"https://github.com/tiagovilasboas/downshift/issues",
		"sh -s <tag>",
	} {
		if !strings.Contains(r.output, want) {
			t.Errorf("output missing %q:\n%s", want, r.output)
		}
	}
	if strings.Contains(r.output, "Downloading downshift") {
		t.Errorf("must not start the download:\n%s", r.output)
	}
	if _, err := os.Stat(filepath.Join(r.binDir, "downshift")); err == nil {
		t.Error("binary was installed despite the missing archive")
	}
	entries, _ := os.ReadDir(r.binDir) // not even created
	if len(entries) != 0 {
		t.Errorf("install dir must stay empty, has %d entries", len(entries))
	}
}

// Regression: an older downshift earlier on PATH (~/go/bin) shadowed the new
// install, and the installer still reported success. It must name both paths.
func TestInstallWarnsWhenAnOlderBinaryShadowsTheNewOne(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"STALE_DOWNSHIFT=1"})
	assertInstalled(t, r)
	shadow := filepath.Join(r.home, "fakebin", "downshift")
	for _, want := range []string{"Another downshift comes first on your PATH: " + shadow, filepath.Join(r.binDir, "downshift"), "ln -sf"} {
		if !strings.Contains(r.output, want) {
			t.Errorf("output lacks %q:\n%s", want, r.output)
		}
	}
}
