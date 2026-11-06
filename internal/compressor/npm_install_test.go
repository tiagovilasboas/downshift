// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func installPad(prefix string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(prefix + " pkg" + string(rune('a'+i%26)) + " 1.0." + string(rune('0'+i%10)) + "\n")
	}
	return b.String()
}

func TestCompressInstallSummaries(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"npm", "added 120 packages, and audited 121 packages in 4s\n\n30 packages are looking for funding\n  run `npm fund` for details\n\nfound 0 vulnerabilities\n",
			[]string{"[x] npm: added 120 packages in 4s"}},
		{"npm up to date", "up to date, audited 121 packages in 900ms\n\n30 packages are looking for funding\n  run `npm fund` for details\n\nfound 0 vulnerabilities\n",
			[]string{"[x] npm: up to date in 900ms"}},
		{"pnpm", "Lockfile is up to date, resolution step is skipped\nProgress: resolved 1, reused 0, downloaded 0, added 0\nPackages: +40\n++++++++++++++++++++++++++++++++++++++++\n" +
			"Progress: resolved 40, reused 10, downloaded 30, added 40, done\n\ndependencies:\n" + installPad("+", 40) + "\nDone in 3.2s\n",
			[]string{"[x] pnpm: +40 packages in 3.2s"}},
		{"yarn", "yarn install v1.22.19\ninfo No lockfile found.\n[1/4] Resolving packages...\n[2/4] Fetching packages...\n[3/4] Linking dependencies...\n[4/4] Building fresh packages...\nsuccess Saved lockfile.\nDone in 5.10s.\n",
			[]string{"[x] yarn: installed in 5.10s"}},
		{"bun", "bun install v1.1.0 (abc123de)\n\n" + installPad("+", 40) + "\n 40 packages installed [1.20s]\n",
			[]string{"[x] bun: 40 packages installed in 1.20s"}},
		{"npm one package", "added 1 package, and audited 2 packages in 1s\n\nfound 0 vulnerabilities\n",
			[]string{"[x] npm: added 1 package in 1s"}},
		{"bun one package", "bun install v1.1.0 (abc123de)\n\n" + installPad("+", 1) + "\n 1 package installed [0.20s]\n",
			[]string{"[x] bun: 1 package installed in 0.20s"}},
		{"npm keeps audit findings", "added 120 packages, and audited 121 packages in 4s\n\n3 vulnerabilities (1 moderate, 2 high)\n\nTo address all issues, run:\n  npm audit fix\n",
			[]string{"[x] npm: added 120 packages in 4s", "3 vulnerabilities (1 moderate, 2 high)", "  npm audit fix"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compress([]byte(tc.in), ModeSafe)
			if !res.Applied || res.Format != FormatNpmInstall {
				t.Fatalf("applied=%v format=%s reason=%s", res.Applied, res.Format, res.Reason)
			}
			for _, w := range tc.want {
				if !strings.Contains(string(res.Output), w) {
					t.Errorf("missing %q in %q", w, res.Output)
				}
			}
		})
	}
}

func TestCompressInstallKeepsEveryWarningAndErrorRaw(t *testing.T) {
	ok := "added 120 packages, and audited 121 packages in 4s\n\nfound 0 vulnerabilities\n" + installPad("", 0)
	lines := map[string]string{
		"npm warn deprecated": "npm warn deprecated inflight@1.0.6: This module is not supported\n",
		"npm WARN old style":  "npm WARN ERESOLVE overriding peer dependency\n",
		"npm error":           "npm error code E404\n",
		"npm ERR":             "npm ERR! code ELIFECYCLE\n",
		"pnpm WARN":           " WARN  deprecated glob@7.2.3: Glob versions prior to v9 are no longer supported\n",
		"yarn warning":        "warning left > right@1.0.0: unmet peer dependency\n",
		"yarn error":          "error An unexpected error occurred\n",
		"yarn info incompat":  "info fsevents@2.3.3: The platform \"linux\" is incompatible with this module.\n",
		"bun warn":            "warn: incorrect peer dependency \"react@17\"\n",
		"unknown line":        "something new npm started printing\n",
		"changed packages":    "changed 3 packages, and audited 121 packages in 2s\n",
		"stack frame":         "    at Object.<anonymous> (/app/x.js:1:1)\n",
	}
	for name, extra := range lines {
		t.Run(name, func(t *testing.T) {
			in := ok + extra + installPad("+", 30)
			res := Compress([]byte(in), ModeSafe)
			if res.Applied || string(res.Output) != in {
				t.Fatalf("must stay raw byte for byte (reason=%s)", res.Reason)
			}
			if r := CompressExit([]byte(in), ModeSafe, 1); r.Applied || string(r.Output) != in {
				t.Fatal("non-zero exit must stay raw")
			}
		})
	}
}
