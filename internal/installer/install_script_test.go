// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestInstallScriptShellcheck(t *testing.T) {
	path, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("shellcheck not installed")
	}
	if out, err := exec.Command(path, installScript).CombinedOutput(); err != nil {
		t.Fatalf("shellcheck install.sh: %v\n%s", err, out)
	}
}

// An unknown flag is refused so a typo is not installed as a version tag.
// The legacy --explicit-upshift flag was removed: Downshift never raises a
// tier on its own, so there is nothing to opt into.
func TestInstallRejectsRemovedExplicitUpshiftFlag(t *testing.T) {
	r := runInstall(t, "v1.0.0", "", "--explicit-upshift")
	if r.code == 0 {
		t.Fatalf("installer accepted the removed --explicit-upshift flag:\n%s", r.output)
	}
}

// Without a session allowlist the hook never rewrites, so the installer must
// tell the user to create one instead of finishing a silent no-op install.
func TestInstallExplainsSessionAllowlist(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	if !strings.Contains(r.output, "session-models.json") {
		t.Fatalf("installer must explain session-models.json, output:\n%s", r.output)
	}
}

// The version is spliced into URLs: anything but a strict tag is refused
// before the first network call.
func TestInstallRejectsMalformedVersions(t *testing.T) {
	for _, v := range []string{"latest", "1.0.0", "v1.0", "v1.0.0/../x", "v1.0.0;id", "v1.0.0 ", "v1.0.0-", "v1.*", "../v1.0.0", "v1.0.0\n"} {
		r := runInstall(t, "v1.0.0", "", v)
		if r.code == 0 || !strings.Contains(r.output, "invalid version") {
			t.Errorf("version %q: exit %d, want a refusal\n%s", v, r.code, r.output)
		}
		if len(r.urls) != 0 {
			t.Errorf("version %q reached the network: %v", v, r.urls)
		}
	}
	for _, v := range []string{"v1.2.3", "v0.1.0-beta.1", "v10.20.30-rc.2"} {
		if r := runInstall(t, "", "", v); r.code != 0 {
			t.Errorf("valid version %q refused: %s", v, r.output)
		}
	}
}

// A latest tag read from GitHub is held to the same rule.
func TestInstallRejectsAMalformedLatestTag(t *testing.T) {
	r := runInstall(t, "v1.0.0;id", "")
	if r.code == 0 || !strings.Contains(r.output, "invalid version") {
		t.Fatalf("exit %d\n%s", r.code, r.output)
	}
}

// The closing line says what was actually proven.
func TestInstallSaysWhatWasVerified(t *testing.T) {
	only := runInstall(t, "v1.0.0", "")
	assertInstalled(t, only)
	if !strings.Contains(only.output, "Verified: sha256 only") || !strings.Contains(only.output, "not who published it") {
		t.Errorf("sha-only install must say so:\n%s", only.output)
	}
	full := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_GH=ok"})
	assertInstalled(t, full)
	if !strings.Contains(full.output, "Verified: sha256 and GitHub build provenance") {
		t.Errorf("provenance install must say so:\n%s", full.output)
	}
}
