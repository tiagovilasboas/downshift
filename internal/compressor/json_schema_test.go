// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

// bigJSON is a document of n records under "items", each with a secret-looking
// value that the schema view must not show.
func bigJSON(n int) string {
	var b strings.Builder
	b.WriteString("{\n  \"total\": 120,\n  \"items\": [\n")
	for i := 0; i < n; i++ {
		sep := ","
		if i == n-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "    {\"id\": %d, \"name\": \"record number %d\", \"token\": \"sk-live-%d\", \"tags\": [\"a\", \"b\"], \"owner\": {\"login\": \"u%d\"}}%s\n", i, i, i, i, sep)
	}
	b.WriteString("  ],\n  \"ok\": true\n}\n")
	return b.String()
}

func TestJSONSchemaView(t *testing.T) {
	in := bigJSON(120)
	out, ok, _ := compressFileRead([]byte(in), CommandHint{Name: "data.json", First: 1})
	if !ok {
		t.Fatalf("not compacted:\n%.200s", out)
	}
	text := string(out)
	for _, want := range []string{
		"{object, 3 keys}\n",
		"  total: number\n",
		"  items: array[120] of {object, 5 keys}\n",
		"    id: number\n",
		"    tags: array[2] of string\n",
		"    owner: {object, 1 keys}\n",
		"      login: string\n",
		"  ok: boolean\n",
		"full text: cat data.json]\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("schema lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "sk-live") || strings.Contains(text, "record number") {
		t.Errorf("schema shows a value:\n%s", text)
	}
	if len(out)*5 > len(in) {
		t.Errorf("schema is %d of %d bytes", len(out), len(in))
	}
}

func TestJSONSchemaJQNamesItsFullCommand(t *testing.T) {
	out, ok, _ := compressFileRead([]byte(bigJSON(120)), CommandHint{Name: "any.dat", JQ: true, First: 1})
	if !ok || !strings.Contains(string(out), "full text: jq . any.dat]") {
		t.Errorf("ok=%v:\n%.300s", ok, out)
	}
}

func TestJSONSchemaStaysRaw(t *testing.T) {
	big := bigJSON(120)
	cases := []struct {
		name, in string
		hint     CommandHint
		cause    string
	}{
		{"small document", bigJSON(3), CommandHint{Name: "a.json"}, "no_compression_gain"},
		{"tail of a file", big, CommandHint{Name: "a.json", Tail: true}, "json_partial_preserved"},
		{"starts past line 1", big, CommandHint{Name: "a.json", First: 10}, "json_partial_preserved"},
		{"cut document", big[:len(big)-40], CommandHint{Name: "a.json"}, "json_invalid_preserved"},
		{"two documents", big + big, CommandHint{Name: "a.json"}, "json_invalid_preserved"},
		{"not json", strings.Repeat("plain text line\n", 400), CommandHint{Name: "a.json"}, "json_invalid_preserved"},
		{"too deep", strings.Repeat("[", 80) + strings.Repeat("]", 80) + strings.Repeat(" ", 5000) + "\n", CommandHint{Name: "a.json"}, "json_invalid_preserved"},
	}
	for _, tc := range cases {
		out, ok, reason := compressFileRead([]byte(tc.in), tc.hint)
		if ok || string(out) != tc.in || reason != tc.cause {
			t.Errorf("%s: ok=%v reason=%q (want raw, %q)", tc.name, ok, reason, tc.cause)
		}
	}
}

func TestJSONSchemaLabelsAndLimits(t *testing.T) {
	var keys strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&keys, "\"k%02d\": %q,", i, strings.Repeat("v", 80))
	}
	in := "{" + keys.String() + "\"mixed\": [1, \"a\", null], \"none\": [], \"nested\": [[1, 2], [3]]}\n"
	out, ok, _ := compressJSONSchema([]byte(in), CommandHint{Name: "a.json"})
	text := string(out)
	for _, want := range []string{"... 23 more keys\n"} {
		if !ok || !strings.Contains(text, want) {
			t.Fatalf("ok=%v lacks %q:\n%s", ok, want, text)
		}
	}
	for _, tc := range []struct{ in, want string }{
		{`{"a":[1,"x",null]}`, "array[3] of mixed"},
		{`{"a":[]}`, "array[0]"},
		{`{"a":[[1,2],[3]]}`, "array[2] of array[2] of number"},
	} {
		root, err := parseJSONShape([]byte(tc.in))
		if err != nil || jsonLabel(root.kids[0]) != tc.want {
			t.Errorf("%s: label %q err %v, want %q", tc.in, jsonLabel(root.kids[0]), err, tc.want)
		}
	}
}
