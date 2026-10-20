// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMavenFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "corpus", name))
	if err != nil {
		t.Fatalf("read Maven fixture %s: %v", name, err)
	}
	return b
}

func TestCompressMavenCompactsCleanBuild(t *testing.T) {
	in := readMavenFixture(t, "maven_pass_package.txt")
	out, applied, reason := compressMaven(in)
	if !applied {
		t.Fatalf("expected compression, reason=%q", reason)
	}
	want := "Maven: inventory 1.4.0 (jar)\n" +
		"  resources:resources (default-resources)\n" +
		"  compiler:compile (default-compile)\n" +
		"  resources:testResources (default-testResources)\n" +
		"  compiler:testCompile (default-testCompile)\n" +
		"  surefire:test (default-test)\n" +
		"  jar:jar (default-jar)\n" +
		"[INFO] Tests run: 13, Failures: 0, Errors: 0, Skipped: 0\n" +
		"[INFO] BUILD SUCCESS\n" +
		"[INFO] Total time:  4.302 s\n"
	if string(out) != want {
		t.Fatalf("unexpected summary:\n%s", out)
	}
}

func TestCompressMavenPreservesAmbiguousOutputByteIdentically(t *testing.T) {
	good := string(readMavenFixture(t, "maven_pass_test.txt"))
	mutate := func(old, repl string) []byte {
		if !strings.Contains(good, old) {
			t.Fatalf("fixture lacks %q", old)
		}
		return []byte(strings.Replace(good, old, repl, 1))
	}
	cases := map[string][]byte{
		"no final newline":    []byte(strings.TrimSuffix(good, "\n")),
		"crlf":                []byte(strings.ReplaceAll(good, "\n", "\r\n")),
		"unknown line":        mutate("[INFO] Results:\n", "[INFO] Results:\nsome test stdout\n"),
		"deprecation":         mutate("[INFO] Results:\n", "[INFO] Results:\n[INFO] Some problems were encountered\n"),
		"download":            mutate("[INFO] Copying 2", "[INFO] Downloading from central: https://x/y.pom\n[INFO] Copying 2"),
		"skipped class":       mutate("Skipped: 0, Time elapsed: 0.012", "Skipped: 1, Time elapsed: 0.012"),
		"total mismatch":      mutate("Tests run: 13,", "Tests run: 14,"),
		"class name mismatch": mutate("-- in com.acme.inventory.PriceTest", "-- in com.acme.Other"),
		"second project":      mutate("[INFO] Scanning for projects...\n", "[INFO] Scanning for projects...\n[INFO] Reactor Build Order:\n"),
		"no success":          mutate("[INFO] BUILD SUCCESS\n", ""),
		"trailing output":     []byte(good + "[INFO] extra\n"),
		"wrong first line":    []byte("[INFO] Hello\n" + good),
	}
	for name, in := range cases {
		out, applied, reason := compressMaven(in)
		if applied || !bytes.Equal(out, in) || !strings.HasPrefix(reason, "maven_") && reason != "no_compression_gain" {
			t.Errorf("%s must fall back byte-identically: applied=%v reason=%q", name, applied, reason)
		}
	}
}

func TestMavenDetectionIsExclusive(t *testing.T) {
	for _, in := range []string{
		"[INFO] Scanning for projects...x\n",
		"Scanning for projects...\n",
		" [INFO] Scanning for projects...\n",
		"BUILD SUCCESS\n",
	} {
		if isMaven([]byte(in)) {
			t.Errorf("must not detect %q", in)
		}
	}
	if !isMaven([]byte("[INFO] Scanning for projects...\n")) {
		t.Error("must detect Maven's first line")
	}
}
