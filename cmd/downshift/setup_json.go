// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// jobj is a JSON object that keeps key order, so a merge into a developer's
// settings file does not reshuffle keys they never touched. Values are *jobj,
// []any, string, json.Number, bool or nil.
type jobj struct {
	keys []string
	vals map[string]any
}

func newJobj() *jobj { return &jobj{vals: map[string]any{}} }

func (o *jobj) get(k string) (any, bool) { v, ok := o.vals[k]; return v, ok }

func (o *jobj) set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func encodeNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (o *jobj) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := encodeNoEscape(k)
		if err != nil {
			return nil, err
		}
		vb, err := encodeNoEscape(o.vals[k])
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// parseJSONDoc parses a settings file. Empty input is an empty object; any
// other non-object or malformed input is an error and nothing is changed.
func parseJSONDoc(b []byte) (*jobj, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return newJobj(), nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := readJSONValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the top-level value")
	}
	o, ok := v.(*jobj)
	if !ok {
		return nil, errors.New("top-level value is not a JSON object")
	}
	return o, nil
}

func readJSONValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, isDelim := tok.(json.Delim)
	if !isDelim {
		return tok, nil
	}
	switch d {
	case '{':
		o := newJobj()
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := kt.(string)
			if !ok {
				return nil, errors.New("object key is not a string")
			}
			v, err := readJSONValue(dec)
			if err != nil {
				return nil, err
			}
			o.set(key, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return o, nil
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := readJSONValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return arr, nil
	}
	return nil, fmt.Errorf("unexpected delimiter %q", d)
}

// renderJSONDoc writes the document with two-space indent and a final newline.
func renderJSONDoc(o *jobj) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// unifiedDiff returns a unified-style diff (3 lines of context) between two
// texts, or "" when they are equal.
func unifiedDiff(oldName, newName, oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a, b := splitLines(oldText), splitLines(newText)
	ops := diffOps(a, b)

	const ctx = 3
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", oldName, newName)
	n := len(ops)
	for i := 0; i < n; {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start := i - ctx
		if start < 0 {
			start = 0
		}
		// Extend the hunk while the next change is within 2*ctx lines.
		last := i
		for k := last + 1; k < n; k++ {
			if ops[k].kind == ' ' {
				continue
			}
			if k-last-1 > 2*ctx {
				break
			}
			last = k
		}
		stop := last + 1 + ctx
		if stop > n {
			stop = n
		}
		aStart, bStart := 1, 1
		aLen, bLen := 0, 0
		if start < n {
			aStart, bStart = ops[start].aLine, ops[start].bLine
		}
		var body strings.Builder
		for _, op := range ops[start:stop] {
			switch op.kind {
			case ' ':
				aLen++
				bLen++
			case '-':
				aLen++
			case '+':
				bLen++
			}
			body.WriteByte(op.kind)
			body.WriteString(op.text)
			body.WriteByte('\n')
		}
		if aLen == 0 {
			aStart--
		}
		if bLen == 0 {
			bStart--
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n%s", aStart, aLen, bStart, bLen, body.String())
		i = stop
	}
	return out.String()
}

type diffOp struct {
	kind         byte // ' ', '-', '+'
	text         string
	aLine, bLine int // 1-based line numbers where this op sits in each side
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// diffOps builds an edit script with a longest-common-subsequence table.
// Settings files are small, so the O(n*m) table is fine.
func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i], i + 1, j + 1})
			i++
			j++
		case j >= m || (i < n && lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, diffOp{'-', a[i], i + 1, j + 1})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j], i + 1, j + 1})
			j++
		}
	}
	return ops
}
