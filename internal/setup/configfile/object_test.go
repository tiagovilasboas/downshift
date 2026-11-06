// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package configfile

import (
	"strings"
	"testing"
)

// A settings file keeps its key order and its characters through a merge.
func TestObjectKeepsKeyOrder(t *testing.T) {
	doc, err := ParseObject([]byte(`{"z":1,"a":{"cmd":"a && b"},"m":[true,null]}`))
	if err != nil {
		t.Fatal(err)
	}
	doc.Set("new", "x")
	doc.Set("z", 2)
	doc.Del("m")
	if inner, ok := AsObject(doc.Value("a")); !ok || inner.Value("cmd") != "a && b" {
		t.Fatalf("nested object = %v", doc.Value("a"))
	}
	if got := strings.Join(doc.Keys(), ","); got != "z,a,new" || doc.Len() != 3 {
		t.Fatalf("keys = %s (len %d)", got, doc.Len())
	}
	out, err := RenderObject(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"z\": 2,\n  \"a\": {\n    \"cmd\": \"a && b\"\n  },\n  \"new\": \"x\"\n}\n"
	if string(out) != want {
		t.Fatalf("render =\n%s\nwant\n%s", out, want)
	}
}

func TestParseObjectRefusesANonObject(t *testing.T) {
	if doc, err := ParseObject([]byte("  \n")); err != nil || doc.Len() != 0 {
		t.Fatalf("empty input = %v, %v; want an empty object", doc, err)
	}
	for _, in := range []string{`[1]`, `{"a":1} {}`, `{"a":`} {
		if _, err := ParseObject([]byte(in)); err == nil {
			t.Errorf("ParseObject(%q) accepted", in)
		}
	}
}

// Editing the servers object leaves every other member's text alone.
func TestRenderWithMembersAtKeepsOtherText(t *testing.T) {
	doc, err := ParseMembers([]byte(`{"big":12345678901234567890,"mcp":{"servers":{"old":{}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	path := []string{"mcp", "servers"}
	servers, err := MembersAt(doc, path)
	if err != nil || MemberIndex(servers, "old") != 0 {
		t.Fatalf("servers = %v, %v", servers, err)
	}
	val, _ := MarshalValue(map[string]string{"command": "a&b"})
	servers = DeleteMember(SetMember(servers, "downshift", val), "old")
	out, err := RenderWithMembersAt(doc, path, servers)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"12345678901234567890", `"a&b"`, `"downshift"`} {
		if !strings.Contains(string(out), s) {
			t.Errorf("render lost %s:\n%s", s, out)
		}
	}
	if strings.Contains(string(out), `"old"`) {
		t.Errorf("deleted member still rendered:\n%s", out)
	}
}

func TestUnifiedDiffMarksTheChange(t *testing.T) {
	if got := UnifiedDiff("a", "b", "x\n", "x\n"); got != "" {
		t.Fatalf("equal texts = %q, want empty", got)
	}
	got := UnifiedDiff("old", "new", "a\nb\n", "a\nc\n")
	want := "--- old\n+++ new\n@@ -1,2 +1,2 @@\n a\n-b\n+c\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
