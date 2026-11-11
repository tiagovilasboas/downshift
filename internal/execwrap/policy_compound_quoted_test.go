// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"strings"
	"testing"
)

// shapeExe is the binary the quoted-shape tables wrap with.
const shapeExe = "/opt/ds/downshift"

// Quoted words inside a compound shape or a chain: only the command that is
// wrapped may hold them, and the decision keeps ReasonAllowlistedQuoted so the
// quoted-rule guard in shellwrap still sees it.
func TestDecide_QuotedInsideShapes(t *testing.T) {
	const w = "'" + shapeExe + "' compact -- "
	cases := []struct {
		name, command string
		want          string
		argv          string
		segments      []string
	}{
		{"run pattern, merge and tail", "go test -run 'A|B' ./... 2>&1 | tail -20",
			w + "'go' 'test' '-run' 'A|B' './...' 2>&1 | tail -20", "go test -run 'A|B' ./...", []string{"go test -run A|B ./...", "tail -20"}},
		{"cd and quoted grep with head", `cd internal && grep -rn "func Decide" . | head -5`,
			"cd internal && " + w + "'grep' '-rn' 'func Decide' '.' | head -5", `grep -rn "func Decide" .`, []string{"grep -rn func Decide .", "cd internal", "head -5"}},
		{"quoted format with merge", `git log --format="%h %s" 2>&1`,
			w + "'git' 'log' '--format=%h %s' 2>&1", `git log --format="%h %s"`, []string{"git log --format=%h %s"}},
		{"quoted pipe char not split", `grep -rn "a | b" . | tail -3`,
			w + "'grep' '-rn' 'a | b' '.' | tail -3", `grep -rn "a | b" .`, []string{"grep -rn a | b .", "tail -3"}},
		{"chain with a quoted segment", "gofmt -l x; go test -run 'TestX' ./pkg 2>&1 | tail -5",
			"gofmt -l x; " + w + "'go' 'test' '-run' 'TestX' './pkg' 2>&1 | tail -5", "go test -run 'TestX' ./pkg", []string{"gofmt -l x", "go test -run TestX ./pkg", "tail -5"}},
		{"chain with devnull on a segment", `git status && grep -rn "x" . 2>/dev/null`,
			w + "git status && '" + shapeExe + "' compact --discard-stderr -- 'grep' '-rn' 'x' '.'", "git status", []string{"grep -rn x .", `grep -rn "x" .`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, shapeExe)
			if !d.Rewrite || d.Command != tc.want || d.Reason != ReasonAllowlistedQuoted || d.Argv != tc.argv || !reflect.DeepEqual(d.Segments, tc.segments) {
				t.Fatalf("Decide(%q) = %+v\nwant command=%q argv=%q segments=%q", tc.command, d, tc.want, tc.argv, tc.segments)
			}
		})
	}
}

// Every line here must stay as written: a substitution, a pipe into anything
// but tail/head -N, a file redirect, a quoted word in a part kept as written,
// a trailing form whose meaning the shape changes, or a quote a separator
// splits.
func TestDecide_QuotedInsideShapesRefuses(t *testing.T) {
	for _, command := range []string{
		`go test -run "$(id)" ./... 2>&1 | tail -5`,
		"go test -run 'X' `id` | tail -5",
		`go test -run 'X' ./... | sh`,
		`go test -run 'X' ./... | grep FAIL`,
		`go test -run 'X' ./... | tail -5 | sh`,
		`go test -run 'X' ./... > out.txt`,
		`go test -run 'X' ./... 2>&1 > out.txt`,
		`go test -run 'X' ./... | tail -5 > out.txt`,
		`go test -run 'X' ./... ; rm -rf /`,
		`go test -run 'X' ./... && sh -c 'rm x'`,
		`go test -run 'X' ./... && echo 'done'`,
		`cd 'a b' && go test -run 'X'`,
		`go test -run 'X' ./... | tail '-5'`,
		`go test -run 'X' ./... || true | tail -5`,
		`go build ./... && go test -run 'X' ./... || true`,
		`go test -run 'X' ./... 2>/dev/null 2>&1`,
		`cd x && go test -run 'X' ./... 2>/dev/null 2>&1 | tail -5`,
		`grep -n "a; rm -rf /" .; go test`,
		`grep -n "a && rm x" . && go test`,
		`grep -n "x | tail -5`,
		`go test -run 'X' ./... &`,
		`sudo grep -rn "x" . | tail -5`,
	} {
		if d := Decide(command, shapeExe); d.Rewrite {
			t.Errorf("Decide(%q) rewrote: %+v", command, d)
		}
	}
}

// The session tag lands on every wrapped segment of a chain whose segments
// are quoted rewrites, and never inside a quoted word.
func TestWithSession_QuotedChain(t *testing.T) {
	const tag = "0123abcd"
	d := Decide(`go vet ./... && grep -rn "compact --" . | head -3`, shapeExe)
	if !d.Rewrite {
		t.Fatalf("not wrapped: %+v", d)
	}
	got := WithSession(d.Command, tag)
	if n := strings.Count(got, " compact --session="+tag+" --"); n != 2 {
		t.Fatalf("tagged %d segments, want 2: %s", n, got)
	}
	if !strings.Contains(got, "'compact --'") {
		t.Fatalf("quoted word changed: %s", got)
	}
}
