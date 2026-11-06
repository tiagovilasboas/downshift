// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMetacharClass(t *testing.T) {
	for _, tc := range []struct{ command, want string }{
		{"go test ./... ; echo done", MetaSemicolon},
		{"cd x && go test", MetaAnd},
		{"go test || false", MetaOr},
		{"go test | tail", MetaPipe},
		{"go test &", MetaBackground},
		{"go test &> out", MetaRedirect},
		{"go test $(pwd)", MetaSubst},
		{"go test `pwd`", MetaBacktick},
		{"go test > out", MetaRedirect},
		{"go test < in", MetaRedirect},
		{"go test -run 'A", MetaQuote},
		{"go test -run \"A", MetaQuote},
		{"go test\nls", MetaNewline},
		{"ls *.go", MetaGlob},
		{"ls ~", MetaGlob},
		{"ls a\\ b", MetaBackslash},
		{"(ls)", MetaParen},
		{"ls #x", MetaOther},
		{"go test ./...", ""},
	} {
		if got := MetacharClass(tc.command); got != tc.want {
			t.Errorf("MetacharClass(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
}

func TestMetacharClassesCoverEveryReturn(t *testing.T) {
	known := map[string]bool{}
	for _, c := range MetacharClasses {
		known[c] = true
	}
	for _, cmd := range []string{";", "&&", "||", "|", "&", "$", "`", ">", "'", "\n", "*", "\\", "(", "#"} {
		if c := MetacharClass("go test " + cmd); !known[c] {
			t.Errorf("class %q of %q is not in MetacharClasses", c, cmd)
		}
	}
}

func TestSkipDetail(t *testing.T) {
	for _, tc := range []struct{ name, reason, command, wantClass, wantFamily string }{
		{"pipe after go test", SkipMetachar, "go test ./... | tail", MetaPipe, string(FamilyGoTest)},
		{"env prefix", SkipMetachar, "CI=1 go test ./... && echo", MetaAnd, string(FamilyGoTest)},
		{"unknown family", SkipMetachar, "foo bar | baz", MetaPipe, ""},
		{"other reason has none", SkipDenied, "go test | tail", "", ""},
		{"no metachar", SkipMetachar, "go test ./...", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := SkipDetail(tc.reason, tc.command)
			if c != tc.wantClass || f != tc.wantFamily {
				t.Fatalf("got %q,%q want %q,%q", c, f, tc.wantClass, tc.wantFamily)
			}
		})
	}
}

// The skip log holds the class and family, never the command text.
func TestSkipLogHoldsClassNotCommand(t *testing.T) {
	p := filepath.Join(t.TempDir(), "skips.jsonl")
	class, family := SkipDetail(SkipMetachar, "go test ./secret/path | tail")
	ev := SkipEvent{Reason: SkipMetachar, Harness: "claude-code", Metachar: class, Family: family}
	if err := AppendSkipEvent(p, ev, time.Now()); err != nil {
		t.Fatal(err)
	}
	// An old row without the new fields stays readable.
	if err := AppendSkip(p, SkipMetachar, "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "tail") {
		t.Fatalf("command text leaked: %s", raw)
	}
	skips, err := ReadSkips(p)
	if err != nil || len(skips) != 2 || skips[0].Metachar != MetaPipe || skips[0].Family != "go-test" || skips[1].Metachar != "" {
		t.Fatalf("skips = %+v, err %v", skips, err)
	}
	byClass, _ := SummarizeSkipClasses(p)
	if len(byClass) != 1 || byClass[MetaPipe] != 1 {
		t.Fatalf("by class = %v", byClass)
	}
	byReason, _ := SummarizeSkips(p)
	if byReason[SkipMetachar] != 2 {
		t.Fatalf("by reason = %v", byReason)
	}
}
