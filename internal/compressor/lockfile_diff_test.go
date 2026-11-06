// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func lockDiff(file string, hunk ...string) string {
	return fmt.Sprintf("diff --git a/%[1]s b/%[1]s\nindex 1a2b3c4..5d6e7f8 100644\n--- a/%[1]s\n+++ b/%[1]s\n@@ -1,9 +1,9 @@\n%s\n",
		file, strings.Join(hunk, "\n"))
}

func TestCompressLockfileDiff(t *testing.T) {
	npm := lockDiff("package-lock.json",
		`     "node_modules/left": {`,
		`-      "version": "1.0.0",`,
		`+      "version": "1.1.0",`,
		`-      "resolved": "https://registry.npmjs.org/left/-/left-1.0.0.tgz",`,
		`+      "resolved": "https://registry.npmjs.org/left/-/left-1.1.0.tgz",`,
		`+    "node_modules/fresh": {`,
		`+      "version": "2.0.0",`,
		`-    "node_modules/gone": {`,
		`-      "version": "0.1.0",`)
	cases := []struct {
		name  string
		in    string
		want  []string // substrings of the summary
		avoid []string
	}{
		{"npm", npm, []string{"package-lock.json: 1 added, 1 removed, 1 changed", "+ fresh 2.0.0", "- gone 0.1.0", "~ left 1.0.0 -> 1.1.0"}, []string{"resolved"}},
		{"yarn", lockDiff("yarn.lock",
			` "left@^1.0.0":`, `-  version "1.0.0"`, `+  version "1.2.0"`,
			`+"fresh@^2.0.0":`, `+  version "2.0.0"`),
			[]string{"yarn.lock: 1 added, 0 removed, 1 changed", "~ left 1.0.0 -> 1.2.0", "+ fresh 2.0.0"}, nil},
		{"pnpm", lockDiff("pnpm-lock.yaml",
			`-  /left@1.0.0:`, `+  /left@1.1.0(react@18.0.0):`, `+  /@scope/new@3.0.0:`),
			[]string{"pnpm-lock.yaml: 1 added, 0 removed, 1 changed", "+ @scope/new 3.0.0", "~ left 1.0.0 -> 1.1.0"}, nil},
		{"cargo", lockDiff("Cargo.lock",
			` name = "serde"`, `-version = "1.0.1"`, `+version = "1.0.2"`,
			` [[package]]`, `+name = "tiny"`, `+version = "0.3.0"`),
			[]string{"Cargo.lock: 1 added, 0 removed, 1 changed", "~ serde 1.0.1 -> 1.0.2", "+ tiny 0.3.0"}, nil},
		{"unchanged version on both sides", lockDiff("Cargo.lock",
			`-name = "a"`, `-version = "1.0.0"`, `+name = "a"`, `+version = "1.0.0"`, `+name = "b"`, `+version = "2.0.0"`),
			[]string{"1 added, 0 removed, 0 changed", "+ b 2.0.0"}, []string{"a 1.0.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			// Pad with unchanged resolved lines so the summary is shorter.
			in += strings.Repeat(" "+`      "integrity": "sha512-abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH",`+"\n", 20)
			in = strings.Replace(in, "@@ -1,9 +1,9 @@", "@@ -1,29 +1,29 @@", 1)
			if f, _ := detectFormat([]byte(in)); f != FormatLockfileDiff {
				t.Fatalf("format = %s", f)
			}
			res := Compress([]byte(in), ModeSafe)
			out := string(res.Output)
			if !res.Applied || res.Format != FormatLockfileDiff {
				t.Fatalf("applied=%v format=%s reason=%s", res.Applied, res.Format, res.Reason)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in\n%s", w, out)
				}
			}
			for _, a := range tc.avoid {
				if strings.Contains(out, a) {
					t.Errorf("unexpected %q in\n%s", a, out)
				}
			}
		})
	}
}

func TestCompressLockfileDiffFailsOpen(t *testing.T) {
	pad := strings.Repeat(" "+`      "integrity": "sha512-abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH",`+"\n", 20)
	body := func(extra string) string {
		return lockDiff("package-lock.json", `     "node_modules/left": {`, `-      "version": "1.0.0",`, `+      "version": "1.1.0",`, extra) + pad
	}
	cases := map[string]string{
		"deprecation line":      body(`+      "deprecated": "use right instead",`),
		"danger word":           body(`+      "note": "build failed upstream",`),
		"token in url":          body(`+      "resolved": "https://u:token123@registry.example/left.tgz",`),
		"non-lockfile file":     lockDiff("package-lock.json", `-a`, `+b`) + lockDiff("README.md", `-a`, `+b`),
		"only metadata changes": lockDiff("package-lock.json", `-      "integrity": "sha512-aaaa",`, `+      "integrity": "sha512-bbbb",`) + pad,
		"unknown line":          lockDiff("yarn.lock", `-  version "1.0.0"`) + "garbage line\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			res := Compress([]byte(in), ModeSafe)
			if res.Format == FormatLockfileDiff && strings.Contains(string(res.Output), "Lockfile diff") {
				t.Fatalf("summarised: %s", res.Output)
			}
			// Every change line must survive whatever the fallback did.
			for _, l := range strings.Split(in, "\n") {
				if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) && !strings.HasPrefix(l, "+++") && !strings.HasPrefix(l, "---") &&
					!strings.Contains(string(res.Output), l) {
					t.Errorf("change line lost: %q", l)
				}
			}
		})
	}
}

func TestCompressLockfileDiffNonZeroExitRaw(t *testing.T) {
	in := lockDiff("Cargo.lock", `-version = "1"`, `+version = "2"`)
	if res := CompressExit([]byte(in), ModeSafe, 1); res.Applied || string(res.Output) != in {
		t.Fatal("non-zero exit must stay raw")
	}
}
