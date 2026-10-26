// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const (
	testBundled  = "/bundle/kiro-cli-chat"
	testOverride = "/override/kiro-cli"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func present(paths ...string) func(string) bool {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

// TestResolveKiroCLI covers the order override, bundled (signed in only), PATH
// name. The sign-in probe is a counting fake: it must run only for the bundled
// copy and only when that copy exists.
func TestResolveKiroCLI(t *testing.T) {
	cases := []struct {
		name       string
		env        map[string]string
		present    []string
		loggedIn   bool
		want       string
		wantProbes int
	}{
		{
			name:     "absolute override that exists wins over a signed-in bundle",
			env:      map[string]string{EnvKiroBin: testOverride},
			present:  []string{testOverride, testBundled},
			loggedIn: true, want: testOverride, wantProbes: 0,
		},
		{
			name:    "relative override is ignored",
			env:     map[string]string{EnvKiroBin: "kiro-cli"},
			present: []string{testBundled}, loggedIn: true,
			want: testBundled, wantProbes: 1,
		},
		{
			name:    "missing override falls through",
			env:     map[string]string{EnvKiroBin: "/gone/kiro-cli"},
			present: []string{testBundled}, loggedIn: true,
			want: testBundled, wantProbes: 1,
		},
		{
			name:    "bundled copy used when signed in",
			present: []string{testBundled}, loggedIn: true,
			want: testBundled, wantProbes: 1,
		},
		{
			name:    "bundled copy skipped when not signed in, falls to PATH name",
			present: []string{testBundled}, loggedIn: false,
			want: "kiro-cli", wantProbes: 1,
		},
		{
			name:    "bundled copy absent: no probe, PATH name",
			present: nil,
			want:    "kiro-cli", wantProbes: 0,
		},
		{
			name:    "nothing found uses the PATH name",
			env:     map[string]string{EnvKiroBin: "relative"},
			present: nil,
			want:    "kiro-cli", wantProbes: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probes := 0
			signedIn := func(bin string) bool {
				probes++
				if bin != testBundled {
					t.Errorf("sign-in probe ran on %q, want only the bundled copy", bin)
				}
				return tc.loggedIn
			}
			got := resolveKiroCLI(envOf(tc.env), testBundled, present(tc.present...), signedIn)
			if got != tc.want {
				t.Errorf("resolveKiroCLI = %q, want %q", got, tc.want)
			}
			if probes != tc.wantProbes {
				t.Errorf("sign-in probes = %d, want %d", probes, tc.wantProbes)
			}
		})
	}
}

// TestResolveKiroCLI_EmptyBundledNeverProbes covers platforms with no bundled
// copy (bundledKiroCLI returns "").
func TestResolveKiroCLI_EmptyBundledNeverProbes(t *testing.T) {
	probes := 0
	got := resolveKiroCLI(envOf(nil), "", present(""), func(string) bool { probes++; return true })
	if got != "kiro-cli" || probes != 0 {
		t.Fatalf("got %q with %d probes, want kiro-cli and no probe", got, probes)
	}
}

// writeFakeWhoami writes a shell stand-in for `kiro-cli-chat whoami`. The body
// is the script text after the shebang. It never touches the real CLI.
func writeFakeWhoami(t *testing.T, body string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kiro-cli-chat")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestSignedInKiroCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-ins need /bin/sh")
	}
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"logged in", `echo "Logged in with Builder ID"`, true},
		{"not logged in, exit 0", `echo "Not logged in"`, false},
		{"logged in text but exit 1", `echo "Logged in with x"; exit 1`, false},
		{"empty output", `exit 0`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := signedInKiroCLI(writeFakeWhoami(t, tc.body)); got != tc.want {
				t.Errorf("signedInKiroCLI = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSignedInKiroCLI_TimeoutCountsAsNotSignedIn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-ins need /bin/sh")
	}
	old := kiroWhoamiTimeout
	kiroWhoamiTimeout = 200 * time.Millisecond
	t.Cleanup(func() { kiroWhoamiTimeout = old })

	start := time.Now()
	got := signedInKiroCLI(writeFakeWhoami(t, `echo "Logged in with x"; sleep 30`))
	if got {
		t.Fatal("a whoami that outlives its timeout must count as not signed in")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("whoami probe not bounded by timeout, took %s", elapsed)
	}
}

func TestSignedInKiroCLI_MissingBinaryIsNotSignedIn(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-kiro-cli")
	if signedInKiroCLI(missing) {
		t.Fatal("a missing binary must not count as signed in")
	}
}
