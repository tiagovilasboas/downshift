// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"testing"
)

func TestDecide_CompoundShapes(t *testing.T) {
	const w = "'/opt/ds/downshift' compact -- "
	cases := []struct {
		name, command string
		rewrite       bool
		want          string
		segments      []string
		reason        string
	}{
		{"cd then go test", "cd internal/core && go test ./...", true, "cd internal/core && " + w + "go test ./...", []string{"cd internal/core"}, ReasonAllowlistedCompound},
		{"cd absolute", "cd /Users/x/repo && git status", true, "cd /Users/x/repo && " + w + "git status", []string{"cd /Users/x/repo"}, ReasonAllowlistedCompound},
		{"pipe tail", "go test ./... | tail -20", true, w + "go test ./... | tail -20", []string{"tail -20"}, ReasonAllowlistedCompound},
		{"pipe head -n", "go build ./... | head -n 5", true, w + "go build ./... | head -n 5", []string{"head -n 5"}, ReasonAllowlistedCompound},
		{"pipe tail -nN", "git status | tail -n30", true, w + "git status | tail -n30", []string{"tail -n30"}, ReasonAllowlistedCompound},
		{"stderr dup and tail", "go test ./... 2>&1 | tail -40", true, w + "go test ./... 2>&1 | tail -40", []string{"tail -40"}, ReasonAllowlistedCompound},
		{"cd, stderr dup and tail", "cd cmd && go test -count=1 ./... 2>&1 | tail -5", true, "cd cmd && " + w + "go test -count=1 ./... 2>&1 | tail -5", []string{"cd cmd", "tail -5"}, ReasonAllowlistedCompound},
		{"extra spaces normalised", "cd  x  &&  go  test   |  tail  -3", true, "cd x && " + w + "go test | tail -3", []string{"cd x", "tail -3"}, ReasonAllowlistedCompound},

		{"cd then not allowlisted", "cd x && sudo ls", false, "", nil, ReasonNotAllowlisted},
		{"pipe after not allowlisted", "sudo ls | tail -5", false, "", nil, ReasonNotAllowlisted},
		{"or list", "go test || true", false, "", nil, ReasonUnsafeChar},
		{"cd tilde", "cd ~/repo && go test", false, "", nil, ReasonUnsafeChar},
		{"cd variable", "cd $HOME && go test", false, "", nil, ReasonUnsafeChar},
		{"cd quoted", "cd 'a b' && go test", false, "", nil, ReasonUnsafeChar},
		{"pipe to grep", "go test ./... | grep FAIL", false, "", nil, ReasonUnsafeChar},
		{"two pipes", "go test | tail -5 | head -2", false, "", nil, ReasonUnsafeChar},
		{"tail follow", "go test | tail -f", false, "", nil, ReasonUnsafeChar},
		{"tail plus", "go test | tail -n +5", false, "", nil, ReasonUnsafeChar},
		{"tail with file", "go test | tail -5 out.txt", false, "", nil, ReasonUnsafeChar},
		{"tail count too long", "go test | tail -123456", false, "", nil, ReasonUnsafeChar},
		{"redirect to file", "go test > out.txt | tail -5", false, "", nil, ReasonUnsafeChar},
		{"stdout dup", "go test 1>&2 | tail -5", false, "", nil, ReasonUnsafeChar},
		{"substitution in core", "cd x && go test $(pwd)", false, "", nil, ReasonUnsafeChar},
		{"already wrapped", "cd x && downshift compact -- go test", false, "", nil, ReasonAlreadyWrapped},
		{"background", "go test & | tail -5", false, "", nil, ReasonUnsafeChar},
		{"no spaces around pipe", "go test|tail -5", false, "", nil, ReasonUnsafeChar},
		{"git global option", "cd x && git -C y status", false, "", nil, ReasonNotAllowlisted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite != tc.rewrite || d.Command != tc.want || d.Reason != tc.reason || !reflect.DeepEqual(d.Segments, tc.segments) {
				t.Fatalf("Decide(%q) = %+v, want rewrite=%v command=%q segments=%q reason=%s", tc.command, d, tc.rewrite, tc.want, tc.segments, tc.reason)
			}
		})
	}
}

// The session tag lands on the wrapped command inside a compound, never on
// the cd prefix or the pipe.
func TestWithSession_Compound(t *testing.T) {
	d := Decide("cd x && go test 2>&1 | tail -5", exe)
	got := WithSession(d.Command, hashA)
	want := "cd x && '/opt/ds/downshift' compact --session=" + hashA + " -- go test 2>&1 | tail -5"
	if got != want {
		t.Fatalf("WithSession = %q, want %q", got, want)
	}
}
