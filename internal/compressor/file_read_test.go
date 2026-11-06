// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

// goSource builds a synthetic Go file of n funcs of 11 lines each.
func goSource(funcs int) string {
	var b strings.Builder
	b.WriteString("package demo\n\nimport \"strings\"\n\n")
	for i := 0; i < funcs; i++ {
		fmt.Fprintf(&b, "func Step%d(s string) string {\n", i)
		for j := 0; j < 8; j++ {
			fmt.Fprintf(&b, "\ts = strings.TrimSpace(s) + \"%d\"\n", j)
		}
		b.WriteString("\treturn s\n}\n")
	}
	return b.String()
}

func TestCompressFileReadFoldsLongSource(t *testing.T) {
	in := goSource(40) // 4 + 40*11 = 444 lines
	out, ok, reason := compressFileRead([]byte(in), CommandHint{Name: "demo.go", First: 1})
	if !ok {
		t.Fatalf("not applied: %s", reason)
	}
	text := string(out)
	for _, want := range []string{
		"1: package demo\n",                     // first line
		"5: func Step0(s string) string {\n",    // declaration, with its file line number
		"434: func Step39(s string) string {\n", // last declaration
		"444: }\n",                              // last line
		"[downshift read: ",                     // closing note
		"full text: sed -n '1,444p' demo.go]\n", // exact range command
		"lines folded]\n",                       // gap markers
	} {
		if !strings.Contains(text, want) {
			t.Errorf("view lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `+ "5"`) {
		t.Errorf("a folded body line survived:\n%s", text)
	}
	if len(out)*100 >= len(in)*fileReadMaxRatio {
		t.Errorf("view is %d of %d bytes", len(out), len(in))
	}
}

func TestCompressFileReadStaysRaw(t *testing.T) {
	long := goSource(40)
	cases := []struct {
		name  string
		in    string
		hint  CommandHint
		cause string
	}{
		{"short file", goSource(5), CommandHint{Name: "demo.go"}, "no_compression_gain"},
		{"just under the threshold", goSource(26), CommandHint{Name: "demo.go"}, "no_compression_gain"},
		{"not a source extension", long, CommandHint{Name: "data.csv"}, "file_read_not_source_preserved"},
		{"no extension", long, CommandHint{Name: "Makefile"}, "file_read_not_source_preserved"},
		{"no name", long, CommandHint{}, "file_read_not_source_preserved"},
		{"env file name", long, CommandHint{Name: ".env"}, "file_read_not_source_preserved"},
		{"no declarations", strings.Repeat("\tstep()\n", 400), CommandHint{Name: "demo.go"}, "file_read_no_outline_preserved"},
	}
	for _, tc := range cases {
		out, ok, reason := compressFileRead([]byte(tc.in), tc.hint)
		if ok || string(out) != tc.in || reason != tc.cause {
			t.Errorf("%s: ok=%v reason=%q (want raw, %q)", tc.name, ok, reason, tc.cause)
		}
	}
}

func TestCompressFileReadKeepsDangerLinesWhole(t *testing.T) {
	long := strings.Repeat("x", 300)
	in := strings.Replace(goSource(40), "\ts = strings.TrimSpace(s) + \"3\"\n", "\treturn fmt.Errorf(\"boom "+long+"\")\n", 1)
	out, ok, _ := compressFileRead([]byte(in), CommandHint{Name: "demo.go"})
	if !ok || !strings.Contains(string(out), "boom "+long+"\")") {
		t.Fatalf("danger line cut or dropped (ok=%v)", ok)
	}
}

func TestCompressFileReadLineNumbersAndCommands(t *testing.T) {
	in := []byte(goSource(40))
	out, _, _ := compressFileRead(in, CommandHint{Name: "'my file.go'", First: 100})
	text := string(out)
	if !strings.Contains(text, "100: package demo\n") || !strings.Contains(text, "full text: sed -n '100,543p' 'my file.go']") {
		t.Errorf("sed offset not applied:\n%s", text[:200])
	}
	out, _, _ = compressFileRead(in, CommandHint{Name: "demo.go", Tail: true})
	if !strings.Contains(string(out), "full text: tail -n 444 demo.go (numbers count from the first line shown)]") {
		t.Errorf("tail hint missing:\n%s", out)
	}
}

func TestCompressFileReadMarkdownHeadings(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n", i)
		for j := 0; j < 8; j++ {
			fmt.Fprintf(&b, "Paragraph text for section %d, line %d.\n", i, j)
		}
	}
	out, ok, _ := compressFileRead([]byte(b.String()), CommandHint{Name: "guide.md"})
	if !ok || !strings.Contains(string(out), "## Section 39") || strings.Contains(string(out), "section 20, line 5.") {
		t.Fatalf("markdown outline wrong (ok=%v):\n%s", ok, out)
	}
}

func TestCutLineKeepsRunes(t *testing.T) {
	s := strings.Repeat("é", 150) // 300 bytes
	got := cutLine(s)
	if !strings.HasSuffix(got, "...") || strings.ContainsRune(got, '\uFFFD') {
		t.Errorf("cutLine broke a rune: %q", got)
	}
}
