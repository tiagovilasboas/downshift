// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sumOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestArchiveName(t *testing.T) {
	for _, tc := range []struct{ tag, goos, goarch, want string }{
		{"v0.14.0", "darwin", "arm64", "downshift_0.14.0_darwin_arm64.tar.gz"},
		{"v0.14.0", "linux", "amd64", "downshift_0.14.0_linux_amd64.tar.gz"},
		{"0.14.0", "windows", "amd64", "downshift_0.14.0_windows_amd64.zip"},
	} {
		got, err := ArchiveName(tc.tag, tc.goos, tc.goarch)
		if err != nil || got != tc.want {
			t.Errorf("ArchiveName(%s,%s,%s) = %q, %v; want %q", tc.tag, tc.goos, tc.goarch, got, err, tc.want)
		}
	}
	if _, err := ArchiveName("v0.14.0", "windows", "arm64"); err == nil {
		t.Error("an unpublished platform must be an error")
	}
}

func TestChecksums(t *testing.T) {
	data := []byte("archive bytes")
	sum := sumOf(data)
	list := []byte("deadbeef  other.zip\n" + sum + "  downshift_0.14.0_linux_amd64.tar.gz\n")
	got, err := ExpectedSum(list, "downshift_0.14.0_linux_amd64.tar.gz")
	if err != nil || got != sum {
		t.Fatalf("ExpectedSum = %q, %v", got, err)
	}
	if err := VerifySum(data, got); err != nil {
		t.Fatal(err)
	}
	if err := VerifySum([]byte("tampered"), got); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("a tampered archive must fail: %v", err)
	}
	if _, err := ExpectedSum(list, "missing.tar.gz"); err == nil {
		t.Error("a missing entry must fail")
	}
	if _, err := ExpectedSum(list, "other.zip"); err == nil {
		t.Error("a short sum must fail")
	}
}

func TestExtractBinary(t *testing.T) {
	tgz := tarGz(t, map[string]string{"README.md": "doc", "downshift": "ELF"})
	if got, err := ExtractBinary(tgz, "x.tar.gz", "downshift"); err != nil || string(got) != "ELF" {
		t.Fatalf("tar.gz: %q, %v", got, err)
	}
	z := zipOf(t, map[string]string{"LICENSE": "lic", "downshift.exe": "MZ"})
	if got, err := ExtractBinary(z, "x.zip", "downshift.exe"); err != nil || string(got) != "MZ" {
		t.Fatalf("zip: %q, %v", got, err)
	}
	if _, err := ExtractBinary(tgz, "x.tar.gz", "downshift.exe"); err == nil {
		t.Error("a missing binary must fail")
	}
	if _, err := ExtractBinary([]byte("not an archive"), "x.tar.gz", "downshift"); err == nil {
		t.Error("garbage must fail")
	}
}
