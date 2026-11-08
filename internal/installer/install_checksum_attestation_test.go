// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256Of(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// The installer downloads checksums.txt and verifies the archive before it
// installs anything. A wrong digest or a missing checksums file is a hard
// stop unless the user opts out.
func TestInstallVerifiesChecksum(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	found := false
	for _, u := range r.urls {
		if strings.HasSuffix(u, "/releases/download/v1.0.0/checksums.txt") {
			found = true
		}
	}
	if !found || !strings.Contains(r.output, "Checksum verified") {
		t.Fatalf("checksums.txt not fetched or not verified; urls=%v\noutput:\n%s", r.urls, r.output)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_BAD_SHA=" + strings.Repeat("0", 64)})
	if r.code != 1 || !strings.Contains(r.output, "checksum mismatch") {
		t.Fatalf("exit %d, want 1 with a mismatch message\noutput:\n%s", r.code, r.output)
	}
	if _, err := os.Stat(filepath.Join(r.binDir, "downshift")); err == nil {
		t.Fatal("binary must not be installed on a checksum mismatch")
	}
}

func TestInstallMissingChecksumsNeedsOptOut(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_NO_CHECKSUMS=1"})
	if r.code != 1 || !strings.Contains(r.output, "DOWNSHIFT_SKIP_CHECKSUM=1") {
		t.Fatalf("exit %d, want 1 naming the opt-out\noutput:\n%s", r.code, r.output)
	}
	r = runInstallEnv(t, "v1.0.0", "", []string{"FAKE_NO_CHECKSUMS=1", "DOWNSHIFT_SKIP_CHECKSUM=1"})
	assertInstalled(t, r)
	if !strings.Contains(r.output, "checksum verification skipped") {
		t.Fatalf("opt-out must be loud\noutput:\n%s", r.output)
	}
}

// Build provenance is checked when gh exists and never blocks the install
// unless the operator asks for it.
func TestInstallVerifiesAttestationWithGH(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_GH=ok"})
	assertInstalled(t, r)
	if !strings.Contains(r.output, "Build provenance verified") {
		t.Fatalf("attestation not reported\noutput:\n%s", r.output)
	}
	log, _ := os.ReadFile(r.ghLog)
	if !strings.Contains(string(log), "attestation verify") || !strings.Contains(string(log), "--repo tiagovilasboas/downshift-ai") {
		t.Fatalf("gh called as %q", log)
	}
}

func TestInstallWarnsButInstallsWhenAttestationFails(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	if !strings.Contains(r.output, "could not be verified") || !strings.Contains(r.output, "sha256 check only") {
		t.Fatalf("expected a warning\noutput:\n%s", r.output)
	}
}

func TestInstallWarnsButInstallsWithoutGH(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"NO_GH=1"})
	assertInstalled(t, r)
	if !strings.Contains(r.output, "gh CLI is not installed") || !strings.Contains(r.output, "Checksum verified") {
		t.Fatalf("expected a warning and the sha256 pass\noutput:\n%s", r.output)
	}
}

func TestInstallRequireAttestationBlocks(t *testing.T) {
	for name, env := range map[string][]string{
		"gh fails":   {"DOWNSHIFT_REQUIRE_ATTESTATION=1"},
		"gh missing": {"DOWNSHIFT_REQUIRE_ATTESTATION=1", "NO_GH=1"},
	} {
		r := runInstallEnv(t, "v1.0.0", "", env)
		if r.code != 1 || !strings.Contains(r.output, "DOWNSHIFT_REQUIRE_ATTESTATION=1") {
			t.Errorf("%s: exit %d, want 1\noutput:\n%s", name, r.code, r.output)
		}
		if _, err := os.Stat(filepath.Join(r.binDir, "downshift")); err == nil {
			t.Errorf("%s: binary installed despite the requirement", name)
		}
	}
	ok := runInstallEnv(t, "v1.0.0", "", []string{"DOWNSHIFT_REQUIRE_ATTESTATION=1", "FAKE_GH=ok"})
	assertInstalled(t, ok)
}

func TestInstallSkipAttestationDoesNotCallGH(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"DOWNSHIFT_SKIP_ATTESTATION=1"})
	assertInstalled(t, r)
	if _, err := os.Stat(r.ghLog); err == nil {
		t.Fatal("gh was called although the check was skipped")
	}
}
