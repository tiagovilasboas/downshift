// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readCorpus(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCompressNpmAudit(t *testing.T) {
	in := readCorpus(t, "npm_audit_clean.txt")
	res := Compress([]byte(in), ModeSafe)
	out := string(res.Output)
	if !res.Applied || res.Format != FormatNpmAudit {
		t.Fatalf("applied=%v format=%s reason=%s", res.Applied, res.Format, res.Reason)
	}
	// Every line of a high or critical block survives verbatim.
	for _, want := range []string{
		"lodash  <4.17.21", "Severity: high",
		"Command Injection in lodash - https://github.com/advisories/GHSA-35jh-r3h4-6jhm",
		"node_modules/lodash", "5 vulnerabilities (2 low, 2 moderate, 1 high)", "  npm audit fix",
		"2 moderate (advisories cut): semver, minimist", "2 low (advisories cut): tough-cookie, word-wrap",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	for _, cut := range []string{"Prototype Pollution in minimist", "GHSA-c2qf-rxjj-qqgw"} {
		if strings.Contains(out, cut) {
			t.Errorf("moderate advisory %q should be cut", cut)
		}
	}
}

func TestCompressNpmAuditCriticalOnlyKeepsEverything(t *testing.T) {
	in := "# npm audit report\n\nx  <1\nSeverity: critical\nRCE in x - https://github.com/advisories/GHSA-aaaa-bbbb-cccc\nNo fix available\nnode_modules/x\n\n1 vulnerability (1 critical)\n"
	if res := Compress([]byte(in), ModeSafe); res.Applied {
		t.Fatalf("nothing to cut, must stay raw: %q", res.Output)
	}
}

func TestCompressNpmAuditFailsOpen(t *testing.T) {
	clean := readCorpus(t, "npm_audit_clean.txt")
	cases := map[string]string{
		"warn line":     readCorpus(t, "npm_audit_warn.txt"),
		"error line":    strings.Replace(clean, "\n5 vulnerabilities", "\nnpm ERR! audit endpoint returned an error\n5 vulnerabilities", 1),
		"unknown field": strings.Replace(clean, "Severity: low", "Severity: low\nRisk: unknown", 1),
		"bad severity":  strings.Replace(clean, "Severity: low", "Severity: bogus", 1),
		"unknown tail":  clean + "Run something else\n",
		"no final nl":   strings.TrimSuffix(clean, "\n"),
		"crlf":          strings.ReplaceAll(clean, "\n", "\r\n"),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if res := Compress([]byte(in), ModeSafe); res.Applied || string(res.Output) != in {
				t.Fatalf("must stay raw, reason=%s", res.Reason)
			}
		})
	}
	if res := CompressExit([]byte(clean), ModeSafe, 1); res.Applied {
		t.Fatal("non-zero exit (vulnerabilities found) must stay raw")
	}
}
