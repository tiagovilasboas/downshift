// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRedact_SecretShapes(t *testing.T) {
	cases := []string{
		"Authorization: Basic dXNlcjpwYXNz",
		"Set-Cookie: session=abc123",
		"dsn=postgres://user:hunter2@db:5432/app",
		"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
		"using ghp_abcdefABCDEF0123456789",
		"OPENAI sk-proj0123456789",
		"key AKIAABCDEFGHIJKLMNOP",
		"DB_PASS=hunter2",
		"NPM_AUTH = xyz",
		"export GITHUB_TOKEN=x",
		"token: abc123",
		"client_secret = xyz",
		"Cookie: sid=1",
	}
	for _, c := range cases {
		if got := string(redact([]byte("ok\n" + c + "\nok"))); got != "ok\n"+redactedLine+"\nok" {
			t.Errorf("%q not withheld: %q", c, got)
		}
	}
	if got := string(redact([]byte("PASS ok  pkg 0.1s\n--- PASS: TestX"))); strings.Contains(got, redactedLine) {
		t.Errorf("test output withheld: %q", got)
	}
}

// Ordinary tool output that merely mentions a credential word is kept: the
// agent retrieves it to read exactly these lines.
func TestRedact_KeepsOrdinaryOutput(t *testing.T) {
	keep := []string{
		"--- PASS: TestTokenBudget (0.00s)",
		"internal/ccr/secret.go:12:5: declared and not used",
		"saved 12400 tokens this session",
		"FAIL: TestCookieJar",
		"ok  \tgithub.com/x/authorization\t0.1s",
		"tokens: 12400 saved", // a figure, but the shape is name: value; see below
	}
	for _, l := range keep[:5] {
		if got := string(redact([]byte(l))); got != l {
			t.Errorf("%q withheld: %q", l, got)
		}
	}
	// "tokens: N" has the shape of an assignment and is withheld on purpose:
	// a false positive here is cheaper than a leaked token value.
	if got := string(redact([]byte(keep[5]))); got != redactedLine {
		t.Errorf("assignment-shaped line kept: %q", got)
	}
}

func TestRedact_PEMBody(t *testing.T) {
	in := "before\n-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\nAAAABG5vbmUAAAAEbm9uZQ\n-----END OPENSSH PRIVATE KEY-----\nafter"
	got := strings.Split(string(redact([]byte(in))), "\n")
	if got[0] != "before" || got[5] != "after" {
		t.Fatalf("context lost: %q", got)
	}
	for _, l := range got[1:5] {
		if l != redactedLine {
			t.Fatalf("PEM line kept: %q", got)
		}
	}
}

func TestStore_RefusesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	target := filepath.Join(base, "elsewhere")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "ccr")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Dir: link}).Put([]byte("x")); err == nil {
		t.Fatal("Put wrote through a symlinked dir")
	}

	s := Store{Dir: filepath.Join(base, "real"), TTL: time.Hour}
	id, err := s.Put([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret.txt")
	if err := os.WriteFile(secret, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := s.path(id)
	_ = os.Remove(p)
	if err := os.Symlink(secret, p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get followed a planted link: %v", err)
	}
	// Put replaces the planted link with a real file instead of reusing it.
	if _, err := s.Put([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(id); err != nil || string(got) != "hello" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestStore_RewritesCorruptEntryAndPrunesTemp(t *testing.T) {
	now := time.Now()
	s := Store{Dir: t.TempDir(), TTL: time.Hour, Now: func() time.Time { return now }}
	id, err := s.Put([]byte("exact bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path(id), []byte("tampered!!!"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(s.Dir, ".put-123.tmp")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-2 * staleTemp)
	_ = os.Chtimes(stale, old, old)
	if _, err := s.Put([]byte("exact bytes")); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(id); string(got) != "exact bytes" {
		t.Fatalf("corrupt entry reused: %q", got)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale temp file not pruned")
	}
}

func TestSlice_GrepTooLong(t *testing.T) {
	if _, err := Slice([]byte("a"), Query{Grep: strings.Repeat("a", MaxGrepBytes+1)}); err == nil {
		t.Fatal("long pattern accepted")
	}
}
