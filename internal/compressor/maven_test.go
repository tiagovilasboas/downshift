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
		"no final newline":       []byte(strings.TrimSuffix(good, "\n")),
		"crlf":                   []byte(strings.ReplaceAll(good, "\n", "\r\n")),
		"unknown line":           mutate("[INFO] Results:\n", "[INFO] Results:\nsome test stdout\n"),
		"deprecation":            mutate("[INFO] Results:\n", "[INFO] Results:\n[INFO] Some problems were encountered\n"),
		"download":               mutate("[INFO] Copying 2", "[INFO] Downloading from central: https://x/y.pom\n[INFO] Copying 2"),
		"skipped class":          mutate("Skipped: 0, Time elapsed: 0.012", "Skipped: 1, Time elapsed: 0.012"),
		"total mismatch":         mutate("Tests run: 13,", "Tests run: 14,"),
		"class name mismatch":    mutate("-- in com.acme.inventory.PriceTest", "-- in com.acme.Other"),
		"second project":         mutate("[INFO] Scanning for projects...\n", "[INFO] Scanning for projects...\n[INFO] Reactor Build Order:\n"),
		"chatter in test region": mutate("-- in com.acme.inventory.StockTest\n", "-- in com.acme.inventory.StockTest\n[INFO] Deleting /home/u/.aws/credentials\n"),
		"javac secret note":      mutate("[INFO] Copying 2", "[INFO] Compiling 1 source file with javac [secret exfil note] to /x\n[INFO] Copying 2"),
		"copy suffix free text":  mutate("[INFO] Copying 2", "[INFO] Copying 2 resources from src to dst leaks text\n[INFO] Copying 2"),
		"chatter after totals":   mutate("Tests run: 13, Failures: 0, Errors: 0, Skipped: 0\n", "Tests run: 13, Failures: 0, Errors: 0, Skipped: 0\n[INFO] Deleting /home/u/.aws/credentials\n"),
		"chatter with spaces":    mutate("[INFO] Copying 2", "[INFO] Deleting /a b/c\n[INFO] Copying 2"),
		"no success":             mutate("[INFO] BUILD SUCCESS\n", ""),
		"trailing output":        []byte(good + "[INFO] extra\n"),
		"wrong first line":       []byte("[INFO] Hello\n" + good),
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

func TestCompressMavenChatterOutsideTestRegionStillCompacts(t *testing.T) {
	good := string(readMavenFixture(t, "maven_pass_test.txt"))
	in := []byte(strings.Replace(good, "[INFO] Copying 2", "[INFO] skip non existing resourceDirectory /work/inventory/src/extra\n[INFO] Copying 2", 1))
	if _, applied, reason := compressMaven(in); !applied {
		t.Fatalf("expected compression, reason=%q", reason)
	}
}
