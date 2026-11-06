// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/ccr"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func keepForTest(t *testing.T, raw string) string {
	t.Helper()
	t.Setenv(paths.EnvStateDir, t.TempDir())
	store, err := ccr.Default()
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Put([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func retrieve(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runRetrieve(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRetrieveReturnsExactLines(t *testing.T) {
	id := keepForTest(t, "one\ntwo FAIL\nthree\nfour FAIL\nfive\n")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"whole", []string{id}, "one\ntwo FAIL\nthree\nfour FAIL\nfive\n"},
		{"grep", []string{id, "--grep", "FAIL"}, "two FAIL\nfour FAIL\n"},
		{"flags first", []string{"--head", "2", id}, "one\ntwo FAIL\n"},
		{"tail", []string{id, "--tail", "1"}, "five\n"},
		{"lines", []string{id, "--lines", "2:3"}, "2:two FAIL\n3:three\n"},
	}
	for _, c := range cases {
		code, out, errb := retrieve(c.args...)
		if code != 0 || out != c.want || errb != "" {
			t.Errorf("%s: code=%d out=%q err=%q, want %q", c.name, code, out, errb, c.want)
		}
	}
}

func TestRetrieveExitCodes(t *testing.T) {
	id := keepForTest(t, "x\n")
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"missing id", nil, 2},
		{"two ids", []string{id, id}, 2},
		{"bad id", []string{"../etc/passwd"}, 2},
		{"bad lines", []string{id, "--lines", "9"}, 2},
		{"reversed lines", []string{id, "--lines", "5:2"}, 2},
		{"negative head", []string{id, "--head", "-1"}, 2},
		{"bad grep", []string{id, "--grep", "("}, 2},
		{"unknown flag", []string{id, "--nope"}, 2},
		{"not found", []string{"000000000000"}, 1},
	}
	for _, c := range cases {
		code, out, errb := retrieve(c.args...)
		if code != c.code || out != "" || errb == "" {
			t.Errorf("%s: code=%d out=%q err=%q, want code %d and a stderr line", c.name, code, out, errb, c.code)
		}
	}
}

func TestRetrieveHelpTopicExists(t *testing.T) {
	if !strings.Contains(helpTopics["retrieve"], "downshift retrieve <id>") {
		t.Error("retrieve has no help entry")
	}
}
