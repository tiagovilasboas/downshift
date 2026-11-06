// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"glpat", "token glpat" + "-Xk3mP9qR7vT2wZ5nB8cD here", "token [redacted] here"},
		{"gldt", "gldt" + "-Qw8Er4Ty2Ui6Op0As3Df", "[redacted]"},
		{"glptt", "trigger=glptt" + "-4f8a1c2e9b7d6035a1b2", "trigger=[redacted]"},
		{"glrt", "(glrt" + "-aB3dE5gH7jK9mN1pQ3sT)", "([redacted])"},
		{"glpat with dash and underscore", "glpat" + "-ab_cd-ef_gh-ij_kl-mn_op", "[redacted]"},
		{"private-token header", "PRIVATE-TOKEN: abc123secret", "PRIVATE-TOKEN: [redacted]"},
		{"private-token lowercase", "private-token=abc123secret x", "private-token=[redacted] x"},
		{"bearer", "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig", "Authorization: Bearer [redacted]"},
		{"url credentials", "git clone https://deploy:s3cret@gitlab.example.com/a/b.git", "git clone https://[redacted]@gitlab.example.com/a/b.git"},
		{"url token only", "https://glpat-notlongenough@host/x", "https://[redacted]@host/x"},
		{"several on a line", "https://u:p@h/ and glpat" + "-Xk3mP9qR7vT2wZ5nB8cD", "https://[redacted]@h/ and [redacted]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := RedactSecrets(tc.in)
			if got != tc.want || !changed {
				t.Errorf("RedactSecrets(%q) = %q, %v; want %q", tc.in, got, changed, tc.want)
			}
		})
	}
}

// Text with no secret is returned as it came, including look-alikes: a token
// prefix with fewer than 20 token characters, an e-mail address, a URL with a
// port and a path.
func TestRedactSecretsLeavesPlainText(t *testing.T) {
	for _, in := range []string{
		"glpat-short", "glrt-123", "mail me at maria@example.com", "https://gitlab.example.com:8443/a/b/-/merge_requests/4",
		"the bearer of bad news", "token: none", "(failed) • #9838",
	} {
		if got, changed := RedactSecrets(in); changed || got != in {
			t.Errorf("RedactSecrets(%q) = %q, %v; want unchanged", in, got, changed)
		}
	}
}

func TestCompressGlabRedactsAndCutsNothingElse(t *testing.T) {
	in := "title:\tfix\n--\nclone https://u:tok3n@gitlab.example.com/a.git\nline two\n"
	out, ok, _ := compressGlab([]byte(in))
	want := "title:\tfix\n--\nclone https://[redacted]@gitlab.example.com/a.git\nline two\n"
	if !ok || string(out) != want {
		t.Fatalf("ok=%v out=%q", ok, out)
	}
	if out, ok, _ = compressGlab([]byte("no secret\n")); ok || string(out) != "no secret\n" {
		t.Errorf("a clean output changed: ok=%v %q", ok, out)
	}
	res := CompressCommandOutputHint([]byte(in), FormatGlab, 0, CommandHint{})
	if !res.Applied || strings.Contains(string(res.Output), "tok3n") {
		t.Errorf("through the command entry: applied=%v\n%s", res.Applied, res.Output)
	}
	if res = CompressCommandOutputHint([]byte(in), FormatGlab, 1, CommandHint{}); res.Applied {
		t.Error("a failed run was compacted")
	}
}
