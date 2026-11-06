// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"bytes"
	"fmt"
	"regexp"
)

// Query selects part of a stored output. From and To are 1-based and
// inclusive; 0 means unset.
type Query struct {
	Grep       string
	Head, Tail int
	From, To   int
}

type numbered struct {
	n    int
	text []byte
}

// MaxGrepBytes caps the length of a grep pattern.
const MaxGrepBytes = 1024

// Slice applies From/To, Grep, Head and Tail, in that order. With no field set
// it returns data unchanged. Lines kept by a range filter carry an "N:" prefix
// with their original number.
func Slice(data []byte, q Query) ([]byte, error) {
	if q == (Query{}) {
		return data, nil
	}
	var re *regexp.Regexp
	if len(q.Grep) > MaxGrepBytes {
		return nil, fmt.Errorf("bad grep pattern: longer than %d bytes", MaxGrepBytes)
	}
	if q.Grep != "" {
		var err error
		if re, err = regexp.Compile(q.Grep); err != nil {
			return nil, fmt.Errorf("bad grep pattern: %w", err)
		}
	}
	lines := splitLines(data)
	if q.From > 0 || q.To > 0 {
		lines = inRange(lines, q.From, q.To)
	}
	if re != nil {
		kept := lines[:0:0]
		for _, l := range lines {
			if re.Match(l.text) {
				kept = append(kept, l)
			}
		}
		lines = kept
	}
	if q.Head > 0 && len(lines) > q.Head {
		lines = lines[:q.Head]
	}
	if q.Tail > 0 && len(lines) > q.Tail {
		lines = lines[len(lines)-q.Tail:]
	}
	return render(lines, q.From > 0 || q.To > 0), nil
}

func splitLines(data []byte) []numbered {
	data = bytes.TrimSuffix(data, []byte("\n"))
	if len(data) == 0 {
		return nil
	}
	parts := bytes.Split(data, []byte("\n"))
	out := make([]numbered, len(parts))
	for i, p := range parts {
		out[i] = numbered{n: i + 1, text: p}
	}
	return out
}

func inRange(lines []numbered, from, to int) []numbered {
	var out []numbered
	for _, l := range lines {
		if (from == 0 || l.n >= from) && (to == 0 || l.n <= to) {
			out = append(out, l)
		}
	}
	return out
}

func render(lines []numbered, prefix bool) []byte {
	var b bytes.Buffer
	for _, l := range lines {
		if prefix {
			fmt.Fprintf(&b, "%d:", l.n)
		}
		b.Write(l.text)
		b.WriteByte('\n')
	}
	return b.Bytes()
}
