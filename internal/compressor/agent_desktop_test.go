// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func desktopEnv(command, data string) []byte {
	return []byte(`{"version":"2.6","ok":true,"command":"` + command + `","data":` + data + "}\n")
}

// rows pads a tree with enough plain buttons that the view is shorter than
// the JSON, so the case under test is not decided by "no gain".
func rows(sid string, from int) string {
	var b strings.Builder
	for i := from; i < from+12; i++ {
		b.WriteString(`,{"available_actions":["Click"],"name":"Row ` + string(rune('A'+i%26)) + `","ref_id":"@` + sid + `:e` + itoa(i) + `","role":"button"}`)
	}
	return b.String()
}

func TestAgentDesktopViewKeepsUnusualFields(t *testing.T) {
	cases := map[string]string{
		"every short form and its fallback": `{"app":"X","complete":false,"nodes_observed":40,"ref_count":14,"snapshot_id":"s1abc","truncated":true,"tree":{"role":"window","name":"","children":[` +
			`{"available_actions":["AXPress","Show Menu"],"bounds":{"height":20.5,"width":100,"x":-3,"y":1e2},"name":"Save \"draft\"\n2","ref_id":"@s1abc:e1","role":"button","states":["focused","Mixed Case"],"value":"ünïcode <&>"},` +
			`{"bounds":{"x":1,"y":2,"width":3},"children_count":7,"description":"d","dom_classes":["a.b","c"],"native_id":{"kind":"Weird Kind","value":"v"},"placeholder":"p","ref_id":"@sOTHER:e9","role":"group","role_description":"rd","subrole":"AXSub","subtree_truncated":true,"hint":"column 1 of 2"},` +
			`{"name":null,"native_id":{"kind":"runtime_id","value":"42.1"},"role":"text","states":[],"future_field":{"a":[1,2]}},` +
			`{"ref_id":"@s1abc:e2","role":"checkbox","subtree_truncated":false}` + rows("s1abc", 3) + `]}}`,
		"no snapshot id": `{"app":"X","complete":true,"ref_count":1,"snapshot_id":"!bad id!","tree":{"role":"window","children":[{"ref_id":"@!bad id!:e1","role":"button"}` + rows("x", 2) + `]}}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) { assertRoundTrip(t, desktopEnv("snapshot", data)) })
	}
}

func TestAgentDesktopViewReadsSortedTopLevelKeys(t *testing.T) {
	// The MCP server re-serializes the envelope with every key sorted; the
	// view reads keys by name, so the order of the CLI's own output is no
	// assumption either.
	in := []byte(`{"command":"snapshot","data":{"app":"X","complete":true,"ref_count":12,"snapshot_id":"s9","tree":{"role":"window","children":[{"role":"group"}` + rows("s9", 1) + `]}},"ok":true,"version":"2.6"}` + "\n")
	assertRoundTrip(t, in)
}

func TestAgentDesktopViewLeavesOtherOutputRaw(t *testing.T) {
	tree := `{"role":"window","children":[{"role":"group"}` + rows("s1", 1) + `]}`
	cases := map[string]string{
		"error envelope":       `{"version":"2.6","ok":false,"command":"click","error":{"code":"STALE_REF","message":"@s8f3k2p9:e7 not found"}}`,
		"other command":        `{"version":"2.6","ok":true,"command":"list-windows","data":[{"id":"w1","title":"A"}]}`,
		"find --count":         `{"version":"2.6","ok":true,"command":"find","data":{"count":3}}`,
		"two envelopes":        string(desktopEnv("snapshot", `{"snapshot_id":"s1","tree":`+tree+`}`)) + string(desktopEnv("snapshot", `{"snapshot_id":"s1","tree":`+tree+`}`)),
		"role with a space":    string(desktopEnv("snapshot", `{"snapshot_id":"s1","tree":{"role":"menu bar","children":[]}}`)),
		"children not a list":  string(desktopEnv("snapshot", `{"snapshot_id":"s1","tree":{"role":"window","children":{"role":"group"}}}`)),
		"key the grammar owns": string(desktopEnv("snapshot", `{"snapshot_id":"s1","tree":{"role":"window","box":1,"children":[{"role":"group"}`+rows("s1", 1)+`]}}`)),
		"not JSON":             "agent-desktop: command not found\n",
		"no version":           `{"ok":true,"command":"snapshot","data":{"tree":` + tree + `}}`,
		"text after the value": string(desktopEnv("snapshot", `{"snapshot_id":"s1","tree":`+tree+`}`)) + "warning: slow\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			r := CompressCommandOutput([]byte(in), FormatAgentDesktop, 0)
			if r.Applied || string(r.Output) != in {
				t.Fatalf("applied=%v reason=%s, want raw\n%s", r.Applied, r.Reason, r.Output)
			}
		})
	}
}

func TestAgentDesktopViewKeepsAFailedRunRaw(t *testing.T) {
	in := desktopEnv("snapshot", `{"snapshot_id":"s1","tree":{"role":"window","children":[{"role":"group"}`+rows("s1", 1)+`]}}`)
	if r := CompressCommandOutput(in, FormatAgentDesktop, 1); r.Applied {
		t.Fatalf("a non-zero exit was compacted: %s", r.Reason)
	}
}

func TestAgentDesktopViewLegends(t *testing.T) {
	in := desktopEnv("snapshot", `{"app":"Finder","complete":true,"ref_count":13,"snapshot_id":"s8f3k2p9","tree":{"role":"window","name":"Downloads","children":[{"ref_id":"@s8f3k2p9:e1","role":"button","name":"No actions"}`+rows("s8f3k2p9", 2)+`]}}`)
	r := assertRoundTrip(t, in)
	out := string(r.Output)
	for _, want := range []string{
		"refs: eN means @s8f3k2p9:eN\n",
		"actions: an eN line without actions= has Click; actions=- means none\n",
		"  e1 button \"No actions\" actions=-\n",
		"  e2 button \"Row C\"\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view lacks %q:\n%s", want, out)
		}
	}
}
