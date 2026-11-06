// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestCollapseProgress(t *testing.T) {
	cases := []struct {
		name, in, want string
		ok             bool
	}{
		{"growing frames", "a\nfoo 1\rfoo 22\rfoo 333\nb\n", "a\nfoo 333\nb\n", true},
		{"equal length frames", "x 1\rx 2\rx 3\n", "x 3\n", true},
		{"leading cr", "\rabc\n", "abc\n", true},
		{"several lines", "p 1\rp 22\nq 1\rq 22\n", "p 22\nq 22\n", true},
		{"shorter frame leaves residue", "abcdef\rxy\n", "", false},
		{"crlf is a line ending", "one\r\ntwo\r\n", "", false},
		{"no cr", "plain\n", "", false},
		{"escape sequence", "a 1\r\x1b[Ka 22\n", "", false},
		{"tab", "a\t1\ra\t22\n", "", false},
		{"non ascii", "é 1\ré 22\n", "", false},
		{"danger in early frame", "failed 1\rok 22222\n", "", false},
		{"danger in last frame", "ok 1\rwarning 22222\n", "", false},
		{"double cr then shorter", "abc\r\rabcd\n", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := collapseProgress([]byte(tc.in))
			if ok != tc.ok || string(got) != tc.want {
				t.Fatalf("got %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestCompressCollapsesProgressOnly(t *testing.T) {
	in := "start\n" + strings.Repeat("Progress 1\r", 50) + "Progress 333\nend\n"
	res := Compress([]byte(in), ModeSafe)
	if !res.Applied || res.Format != FormatProgress || string(res.Output) != "start\nProgress 333\nend\n" {
		t.Fatalf("applied=%v format=%s out=%q reason=%s", res.Applied, res.Format, res.Output, res.Reason)
	}
	for _, raw := range []string{"abcdef\rxy\n", "a\r\nb\r\n", "ok 1\rfail 22\n"} {
		if r := Compress([]byte(raw), ModeSafe); r.Applied || string(r.Output) != raw {
			t.Errorf("%q must stay raw, got %q (%s)", raw, r.Output, r.Reason)
		}
	}
	if r := CompressExit([]byte(in), ModeSafe, 1); r.Applied {
		t.Error("non-zero exit must stay raw")
	}
	if r := Compress([]byte(in), ModeObserve); string(r.Output) != in {
		t.Error("observe mode must not change the output")
	}
}
