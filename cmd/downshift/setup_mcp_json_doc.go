// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
)

// jsonMember is one key of a JSON object with its value kept as the bytes the
// user wrote. Editing one member and re-emitting the others untouched keeps
// their key order, number text (large integers), and characters such as & < >.
type jsonMember struct {
	key string
	val json.RawMessage
}

var errNotJSONObject = errors.New("not a JSON object")

// parseJSONObject splits a JSON object into its members, in file order.
func parseJSONObject(raw []byte) ([]jsonMember, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errNotJSONObject
	}
	members := []jsonMember{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := kt.(string)
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return nil, err
		}
		members = append(members, jsonMember{key: key, val: val})
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON object")
	}
	return members, nil
}

// renderJSONObject writes the members as an indented object with a final
// newline. Strings are never HTML-escaped and member values are not reparsed.
func renderJSONObject(members []jsonMember) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := marshalJSONValue(m.key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(m.val)
	}
	buf.WriteByte('}')
	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func jsonMemberIndex(members []jsonMember, key string) int {
	for i := len(members) - 1; i >= 0; i-- {
		if members[i].key == key {
			return i
		}
	}
	return -1
}

// setJSONMember replaces key in place, or appends it.
func setJSONMember(members []jsonMember, key string, val json.RawMessage) []jsonMember {
	if i := jsonMemberIndex(members, key); i >= 0 {
		members[i].val = val
		return members
	}
	return append(members, jsonMember{key: key, val: val})
}

func deleteJSONMember(members []jsonMember, key string) []jsonMember {
	out := make([]jsonMember, 0, len(members))
	for _, m := range members {
		if m.key != key {
			out = append(out, m)
		}
	}
	return out
}

// marshalJSONValue encodes v without HTML escaping.
func marshalJSONValue(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// withMcpServers stores servers back under "mcpServers" and renders the file.
func withMcpServers(doc, servers []jsonMember) ([]byte, error) {
	serversRaw, err := renderJSONObject(servers)
	if err != nil {
		return nil, err
	}
	doc = setJSONMember(doc, "mcpServers", json.RawMessage(bytes.TrimSpace(serversRaw)))
	return renderJSONObject(doc)
}

// mcpEntrySummary describes a server entry as command, args and env key names
// only, so the dry-run shows what setup owns and never prints env values.
func mcpEntrySummary(entry map[string]any) string {
	var b strings.Builder
	b.WriteString("mcpServers." + mcpServerName + "\n")
	cmd, _ := entry["command"].(string)
	b.WriteString("  command: " + cmd + "\n")
	var args []string
	if list, ok := entry["args"].([]any); ok {
		for _, a := range list {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}
	b.WriteString("  args: " + strings.Join(args, " ") + "\n")
	var keys []string
	if env, ok := entry["env"].(map[string]any); ok {
		for k := range env {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	b.WriteString("  env keys: " + strings.Join(keys, ", ") + "\n")
	return b.String()
}
