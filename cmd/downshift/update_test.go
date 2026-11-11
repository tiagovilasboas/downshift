// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeReleases struct {
	latest string
	assets map[string][]byte
	gets   []string
}

func (f *fakeReleases) LatestTag() (string, error) {
	if f.latest == "" {
		return "", errors.New("offline")
	}
	return f.latest, nil
}

func (f *fakeReleases) Download(tag, asset string) ([]byte, error) {
	f.gets = append(f.gets, asset)
	b, ok := f.assets[asset]
	if !ok {
		return nil, errors.New("HTTP 404")
	}
	return b, nil
}

func releaseTarGz(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "downshift", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write([]byte(body))
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

type updateFixture struct {
	src  *fakeReleases
	deps updateDeps
	runs []string
	out  bytes.Buffer
	errw bytes.Buffer
}

// newUpdateFixture publishes v0.15.0 for linux/amd64 with a matching sum
// (tamper flips the archive after the sum is computed).
func newUpdateFixture(t *testing.T, current string, tamper bool) *updateFixture {
	t.Helper()
	archive := releaseTarGz(t, "NEW")
	sum := sha256.Sum256(archive)
	if tamper {
		archive = releaseTarGz(t, "EVIL")
	}
	name := "downshift_0.15.0_linux_amd64.tar.gz"
	exe := filepath.Join(t.TempDir(), "downshift")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := &updateFixture{src: &fakeReleases{latest: "v0.15.0", assets: map[string][]byte{
		name:            archive,
		"checksums.txt": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"),
	}}}
	f.deps = updateDeps{Source: f.src, Exe: exe, Current: current, GOOS: "linux", GOARCH: "amd64",
		Run: func(bin string, args ...string) int {
			f.runs = append(f.runs, filepath.Base(bin)+" "+strings.Join(args, " "))
			return 0
		}}
	return f
}

func (f *updateFixture) exeContent(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(f.deps.Exe)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUpdate_InstallsNewerReleaseThenRunsSetupAndDoctor(t *testing.T) {
	f := newUpdateFixture(t, "0.14.0", false)
	if code := update(updateFlags{}, f.deps, &f.out, &f.errw); code != 0 {
		t.Fatalf("exit %d: %s", code, f.errw.String())
	}
	if got := f.exeContent(t); got != "NEW" {
		t.Errorf("binary = %q, want the release", got)
	}
	want := []string{"downshift setup", "downshift doctor"}
	if strings.Join(f.runs, "|") != strings.Join(want, "|") {
		t.Errorf("runs = %v, want %v (setup must ask: no --yes)", f.runs, want)
	}
	if !strings.Contains(f.out.String(), "Checksum verified") {
		t.Errorf("output: %s", f.out.String())
	}
}

func TestUpdate_YesIsPassedToSetup(t *testing.T) {
	f := newUpdateFixture(t, "0.14.0", false)
	update(updateFlags{yes: true}, f.deps, &f.out, &f.errw)
	if len(f.runs) == 0 || f.runs[0] != "downshift setup --yes" {
		t.Errorf("runs = %v", f.runs)
	}
}

func TestUpdate_CurrentBinarySkipsDownloadButRefreshesHarnesses(t *testing.T) {
	f := newUpdateFixture(t, "0.15.0", false)
	if code := update(updateFlags{}, f.deps, &f.out, &f.errw); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(f.src.gets) != 0 || f.exeContent(t) != "OLD" {
		t.Errorf("a current binary must not be downloaded: gets=%v", f.src.gets)
	}
	if len(f.runs) != 2 {
		t.Errorf("setup and doctor must still run: %v", f.runs)
	}
}

func TestUpdate_ChecksumMismatchKeepsOldBinary(t *testing.T) {
	f := newUpdateFixture(t, "0.14.0", true)
	if code := update(updateFlags{}, f.deps, &f.out, &f.errw); code == 0 {
		t.Fatal("a tampered archive must fail")
	}
	if f.exeContent(t) != "OLD" || len(f.runs) != 0 {
		t.Errorf("nothing may change: binary %q, runs %v", f.exeContent(t), f.runs)
	}
	if !strings.Contains(f.errw.String(), "sha256 mismatch") {
		t.Errorf("stderr: %s", f.errw.String())
	}
}

func TestUpdate_CheckChangesNothing(t *testing.T) {
	f := newUpdateFixture(t, "0.14.0", false)
	if code := update(updateFlags{check: true}, f.deps, &f.out, &f.errw); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(f.out.String(), "0.14.0 -> v0.15.0") || len(f.runs) != 0 || len(f.src.gets) != 0 {
		t.Errorf("out=%q runs=%v gets=%v", f.out.String(), f.runs, f.src.gets)
	}
}

func TestUpdate_FailedSetupSkipsDoctorAndReturnsItsCode(t *testing.T) {
	f := newUpdateFixture(t, "0.15.0", false)
	f.deps.Run = func(bin string, args ...string) int {
		f.runs = append(f.runs, args[0])
		return 3
	}
	if code := update(updateFlags{}, f.deps, &f.out, &f.errw); code != 3 || len(f.runs) != 1 {
		t.Errorf("code %d runs %v", code, f.runs)
	}
}

func TestUpdate_OfflineAndUnpublishedPlatformFail(t *testing.T) {
	f := newUpdateFixture(t, "0.14.0", false)
	f.src.latest = ""
	if update(updateFlags{}, f.deps, &f.out, &f.errw) == 0 {
		t.Error("no latest tag must fail")
	}
	f = newUpdateFixture(t, "0.14.0", false)
	f.deps.GOARCH = "riscv64"
	if update(updateFlags{}, f.deps, &f.out, &f.errw) == 0 || f.exeContent(t) != "OLD" {
		t.Error("an unpublished platform must fail and keep the binary")
	}
}

func TestBinaryUpdateNeeded(t *testing.T) {
	for _, tc := range []struct {
		current, tag string
		pinned, want bool
	}{
		{"0.14.0", "v0.15.0", false, true},
		{"0.15.0", "v0.15.0", false, false},
		{"0.16.0", "v0.15.0", false, false},
		{"dev", "v0.15.0", false, true},
		{"0.15.0", "v0.14.0", true, true},
	} {
		if got := binaryUpdateNeeded(tc.current, tc.tag, tc.pinned); got != tc.want {
			t.Errorf("binaryUpdateNeeded(%q,%q,%v) = %v", tc.current, tc.tag, tc.pinned, got)
		}
	}
}

func TestParseUpdateArgs(t *testing.T) {
	var out, errw bytes.Buffer
	f, code := parseUpdateArgs([]string{"--check", "-y", "--version", "0.14.0"}, &out, &errw)
	if code != -1 || !f.check || !f.yes || f.version != "v0.14.0" {
		t.Errorf("got %+v code %d", f, code)
	}
	if _, code := parseUpdateArgs([]string{"--bogus"}, &out, &errw); code != 2 {
		t.Errorf("unknown flag code = %d", code)
	}
}
