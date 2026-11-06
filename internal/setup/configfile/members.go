// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package configfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Member is one key of a JSON object with its value kept as the bytes the
// user wrote. Editing one member and re-emitting the others untouched keeps
// their key order, number text (large integers), and characters such as & < >.
type Member struct {
	Key string
	Val json.RawMessage
}

var errNotObject = errors.New("not a JSON object")

// ParseMembers splits a JSON object into its members, in file order.
func ParseMembers(raw []byte) ([]Member, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errNotObject
	}
	members := []Member{}
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
		members = append(members, Member{Key: key, Val: val})
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON object")
	}
	return members, nil
}

// RenderMembers writes the members as an indented object with a final
// newline. Strings are never HTML-escaped and member values are not reparsed.
func RenderMembers(members []Member) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := MarshalValue(m.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(m.Val)
	}
	buf.WriteByte('}')
	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func MemberIndex(members []Member, key string) int {
	for i := len(members) - 1; i >= 0; i-- {
		if members[i].Key == key {
			return i
		}
	}
	return -1
}

// SetMember replaces key in place, or appends it.
func SetMember(members []Member, key string, val json.RawMessage) []Member {
	if i := MemberIndex(members, key); i >= 0 {
		members[i].Val = val
		return members
	}
	return append(members, Member{Key: key, Val: val})
}

func DeleteMember(members []Member, key string) []Member {
	out := make([]Member, 0, len(members))
	for _, m := range members {
		if m.Key != key {
			out = append(out, m)
		}
	}
	return out
}

// MarshalValue encodes v without HTML escaping.
func MarshalValue(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// MembersAt returns the members of the servers object at path. A missing
// or null object is empty; a value on the way that is not an object is an
// error, so the caller leaves the file as it is.
func MembersAt(doc []Member, path []string) ([]Member, error) {
	i := MemberIndex(doc, path[0])
	if i < 0 || string(doc[i].Val) == "null" {
		return []Member{}, nil
	}
	inner, err := ParseMembers(doc[i].Val)
	if err != nil {
		return nil, err
	}
	if len(path) == 1 {
		return inner, nil
	}
	return MembersAt(inner, path[1:])
}

// withMembersAt returns the members of doc with the servers object at path
// replaced by servers; every other member keeps its text and order.
func withMembersAt(doc []Member, path []string, servers []Member) ([]Member, error) {
	if len(path) == 1 {
		raw, err := RenderMembers(servers)
		if err != nil {
			return nil, err
		}
		return SetMember(doc, path[0], json.RawMessage(bytes.TrimSpace(raw))), nil
	}
	inner := []Member{}
	if i := MemberIndex(doc, path[0]); i >= 0 && string(doc[i].Val) != "null" {
		var err error
		if inner, err = ParseMembers(doc[i].Val); err != nil {
			return nil, err
		}
	}
	inner, err := withMembersAt(inner, path[1:], servers)
	if err != nil {
		return nil, err
	}
	raw, err := RenderMembers(inner)
	if err != nil {
		return nil, err
	}
	return SetMember(doc, path[0], json.RawMessage(bytes.TrimSpace(raw))), nil
}

// RenderWithMembersAt stores servers at path and renders the whole file.
func RenderWithMembersAt(doc []Member, path []string, servers []Member) ([]byte, error) {
	next, err := withMembersAt(doc, path, servers)
	if err != nil {
		return nil, err
	}
	return RenderMembers(next)
}
