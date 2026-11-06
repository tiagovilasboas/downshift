// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

const runSecret = "s3cr3t-session-value"

func TestRun_CurlCapsAndRedactsAndNeverStores(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not installed")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "session="+runSecret)
		w.Header().Set("Authorization", "Bearer "+runSecret)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, strings.Repeat("a line of a long page\n", 400))
	}))
	defer srv.Close()

	var out, errb bytes.Buffer
	stored := false
	keep := func([]byte) (string, bool) { stored = true; return "id", true }
	r := Run([]string{"curl", "-si", srv.URL}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, Keep: keep, KeepFailed: keep})
	text := out.String()
	if !r.Compacted || r.Family != FamilyCurl {
		t.Fatalf("not compacted: compacted=%v family=%q\n%.300s", r.Compacted, r.Family, text)
	}
	if stored {
		t.Error("the raw response, with its Set-Cookie header, was offered to the raw store")
	}
	if strings.Contains(text, runSecret) || strings.Contains(strings.ToLower(text), "set-cookie: session") {
		t.Errorf("compacted output shows the cookie:\n%s", text)
	}
	for _, want := range []string{"HTTP/1.1 200 OK", "Content-Type: text/plain", "Set-Cookie: [redacted]", "Authorization: [redacted]", "bytes cut"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

// A fetch response may carry Set-Cookie, so neither storage port is ever used,
// not even for a failed run (the KeepFailed path).
func TestGateKeeper_FetchFamiliesGetNoStore(t *testing.T) {
	keep := func([]byte) (string, bool) { return "id", true }
	for _, g := range []*gate{{keep: keep, keepFailed: keep}, {keepFailed: keep}} {
		for _, f := range []Family{FamilyCurl, FamilyWget} {
			for _, code := range []int{0, 22} {
				if g.keeper(code, f) != nil {
					t.Errorf("%s exit %d: got a storage port", f, code)
				}
			}
		}
		if g.keeper(22, FamilyGoTest) == nil {
			t.Error("a family outside rawStoreRefused lost its storage port")
		}
	}
}
