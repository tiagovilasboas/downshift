// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DOMAIN: agent-desktop (vercel-labs/agent-desktop) prints one JSON envelope
// per command: {"version","ok","command","data"|"error"}. A snapshot's data
// holds an accessibility tree whose nodes repeat their key names, quote
// marks and the snapshot id inside every ref ("@s8f3k2p9:e12"). The view
// below re-encodes the same facts as one line per node, indented by depth:
//
//	agent-desktop snapshot (envelope 2.6) app="Finder" complete=true ref_count=59 snapshot_id="s8f3k2p9"
//	refs: eN means @s8f3k2p9:eN
//	window "Downloads"
//	  e8 treeitem "Documents" actions=Click
//
// Every field of every node is kept (see agent_desktop_node.go for the
// grammar); the test decodes the view back to the same JSON value. Only a
// successful snapshot or a default find is re-encoded: an error envelope,
// any other command, an unexpected type or a key the grammar cannot name
// leaves the output raw.

// agentDesktopEnvelope is the part of the envelope the view reads.
type agentDesktopEnvelope struct {
	Version string                     `json:"version"`
	OK      bool                       `json:"ok"`
	Command string                     `json:"command"`
	Data    map[string]json.RawMessage `json:"data"`
	Error   json.RawMessage            `json:"error"`
}

// compressAgentDesktop returns the view of one agent-desktop envelope.
func compressAgentDesktop(input []byte) ([]byte, bool, string) {
	env, ok := parseAgentDesktop(input)
	if !ok {
		return input, false, "agent_desktop_unrecognized"
	}
	var out []byte
	switch env.Command {
	case "snapshot":
		out, ok = agentDesktopSnapshotView(env)
	case "find":
		out, ok = agentDesktopFindView(env)
	}
	if !ok {
		return input, false, "agent_desktop_unrecognized"
	}
	if len(out) >= len(input) {
		return input, false, noGainReasonText
	}
	return out, true, "agent-desktop JSON re-encoded one line per element"
}

// noGainReasonText is the reason the other compressors give when a summary
// would not be shorter (execwrap pins it as noGainReason).
const noGainReasonText = "no_compression_gain"

// parseAgentDesktop reads exactly one successful envelope: a second value,
// a failed run or a missing data object is not an output the view knows.
func parseAgentDesktop(input []byte) (agentDesktopEnvelope, bool) {
	var env agentDesktopEnvelope
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	if err := dec.Decode(&env); err != nil {
		return env, false
	}
	if dec.More() || len(bytes.TrimSpace(input[dec.InputOffset():])) > 0 {
		return env, false
	}
	if !env.OK || env.Data == nil || len(env.Error) > 0 || env.Version == "" || !isDesktopToken(env.Command) {
		return env, false
	}
	return env, true
}

// agentDesktopSnapshotView renders a snapshot: the header, the ref legend and
// the tree.
func agentDesktopSnapshotView(env agentDesktopEnvelope) ([]byte, bool) {
	rawTree, ok := env.Data["tree"]
	if !ok {
		return nil, false
	}
	tree, ok := decodeDesktopValue(rawTree).(map[string]any)
	if !ok {
		return nil, false
	}
	cx := desktopCtx{sid: desktopSnapshotID(env.Data)}
	if cx.sid != "" {
		cx.actions = desktopCommonActions(tree, cx.sid)
	}
	var b strings.Builder
	if !writeDesktopHeader(&b, env, "tree") {
		return nil, false
	}
	writeDesktopRefLegend(&b, cx.sid)
	if cx.actions != "" {
		fmt.Fprintf(&b, "actions: an eN line without actions= has %s; actions=- means none\n", cx.actions)
	}
	if !writeDesktopNode(&b, tree, 0, cx) {
		return nil, false
	}
	return []byte(b.String()), true
}

// writeDesktopHeader writes the command line and every data key except skip,
// sorted, as key=<compact JSON>.
func writeDesktopHeader(b *strings.Builder, env agentDesktopEnvelope, skip string) bool {
	fmt.Fprintf(b, "agent-desktop %s (envelope %s)", env.Command, env.Version)
	keys := make([]string, 0, len(env.Data))
	for k := range env.Data {
		if k != skip {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !isDesktopToken(k) {
			return false
		}
		fmt.Fprintf(b, " %s=%s", k, desktopJSON(decodeDesktopValue(env.Data[k])))
	}
	b.WriteByte('\n')
	return true
}

// desktopCommonActions is the action list most eN nodes of tree carry, when
// at least three do; "" otherwise (the legend is then left out).
func desktopCommonActions(tree map[string]any, sid string) string {
	counts := map[string]int{}
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		if r := desktopRef(n["ref_id"], sid); n["ref_id"] != nil && !strings.HasPrefix(r, "ref=") {
			if s, ok := joinDesktopWords(n["available_actions"], reDesktopWord); ok {
				counts[s]++
			}
		}
		kids, _ := n["children"].([]any)
		for _, k := range kids {
			if c, ok := k.(map[string]any); ok {
				walk(c)
			}
		}
	}
	walk(tree)
	best, most := "", 2
	for s, c := range counts {
		if c > most || (c == most && best != "" && s < best) {
			best, most = s, c
		}
	}
	return best
}

// writeDesktopRefLegend names the snapshot every short ref (eN) belongs to.
func writeDesktopRefLegend(b *strings.Builder, sid string) {
	if sid != "" {
		fmt.Fprintf(b, "refs: eN means @%s:eN\n", sid)
	}
}

// desktopSnapshotID is data.snapshot_id when it is a plain id, else "" (and
// refs then stay whole).
func desktopSnapshotID(data map[string]json.RawMessage) string {
	var sid string
	if raw, ok := data["snapshot_id"]; !ok || json.Unmarshal(raw, &sid) != nil || !isDesktopID(sid) {
		return ""
	}
	return sid
}

// decodeDesktopValue decodes one JSON value keeping numbers as written.
func decodeDesktopValue(raw json.RawMessage) any {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return nil
	}
	return v
}

// desktopJSON is v as compact JSON without HTML escaping.
func desktopJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) != nil {
		return "null"
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
