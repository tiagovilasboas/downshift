// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// The schema view of a large JSON document (cat x.json, jq . x.json).
const (
	// JSONSchemaMinBytes is the shortest document that gets a schema view.
	JSONSchemaMinBytes = 4096
	jsonMaxDepth       = 64 // nesting past this stays raw
	jsonShowDepth      = 6  // levels drawn; deeper ones end in "..."
	jsonMaxKeys        = 40 // keys drawn per object
	jsonMaxRatio       = 70 // the view must be under this percent of the input
)

// jsonNode is the shape of one value: its kind, and for an object its keys in
// document order, for an array its length and the shape of its first element.
type jsonNode struct {
	kind  string // object, array, string, number, boolean, null
	keys  []string
	kids  []*jsonNode
	n     int
	elem  *jsonNode
	mixed bool // the elements of an array are not all of one kind
}

// compressJSONSchema replaces a large, single, valid JSON document with its
// shape: every key, the type of each value, the length of each array and the
// shape of its first element. Values are never shown. Text that is short, not
// one JSON document (a head or tail cut, a stream of documents), a command
// that did not print from line 1, or a view that is not shorter stays raw.
func compressJSONSchema(input []byte, h CommandHint) ([]byte, bool, string) {
	if len(input) < JSONSchemaMinBytes {
		return input, false, "no_compression_gain"
	}
	if h.Tail || h.First > 1 {
		return input, false, "json_partial_preserved"
	}
	root, err := parseJSONShape(input)
	if err != nil {
		return input, false, "json_invalid_preserved"
	}
	var b strings.Builder
	b.WriteString(jsonLabel(root) + "\n")
	jsonEmit(&b, root, 1)
	full := "cat " + nameOrFile(h.Name)
	if h.JQ {
		full = "jq . " + nameOrFile(h.Name)
	}
	out := fmt.Sprintf("%s[downshift json: schema of %d bytes, %d lines; full text: %s]\n",
		b.String(), len(input), bytes.Count(input, []byte("\n")), full)
	if len(out)*100 >= len(input)*jsonMaxRatio {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "large JSON document reduced to its schema"
}

func nameOrFile(name string) string {
	if name == "" {
		return "<file>"
	}
	return name
}

// parseJSONShape reads exactly one JSON value from data.
func parseJSONShape(data []byte) (*jsonNode, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	root, err := readJSONValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("data after the first document")
	}
	return root, nil
}

func readJSONValue(dec *json.Decoder, depth int) (*jsonNode, error) {
	if depth > jsonMaxDepth {
		return nil, fmt.Errorf("nested too deep")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			return readJSONObject(dec, depth)
		}
		return readJSONArray(dec, depth)
	case string:
		return &jsonNode{kind: "string"}, nil
	case float64:
		return &jsonNode{kind: "number"}, nil
	case bool:
		return &jsonNode{kind: "boolean"}, nil
	}
	return &jsonNode{kind: "null"}, nil
}

func readJSONObject(dec *json.Decoder, depth int) (*jsonNode, error) {
	n := &jsonNode{kind: "object"}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		child, err := readJSONValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		n.keys = append(n.keys, key.(string))
		n.kids = append(n.kids, child)
	}
	_, err := dec.Token() // the closing brace
	return n, err
}

func readJSONArray(dec *json.Decoder, depth int) (*jsonNode, error) {
	n := &jsonNode{kind: "array"}
	for dec.More() {
		child, err := readJSONValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		if n.elem == nil {
			n.elem = child
		} else if child.kind != n.elem.kind {
			n.mixed = true
		}
		n.n++
	}
	_, err := dec.Token() // the closing bracket
	return n, err
}

// jsonLabel is the one-line description of a value.
func jsonLabel(n *jsonNode) string {
	switch {
	case n.kind == "object":
		return fmt.Sprintf("{object, %d keys}", len(n.keys))
	case n.kind != "array":
		return n.kind
	case n.elem == nil:
		return "array[0]"
	case n.mixed:
		return fmt.Sprintf("array[%d] of mixed", n.n)
	}
	return fmt.Sprintf("array[%d] of %s", n.n, jsonLabel(n.elem))
}

// jsonEmit draws the keys of an object, indented, and then the keys of the
// first element of every array of objects below it.
func jsonEmit(b *strings.Builder, n *jsonNode, level int) {
	for n.kind == "array" && n.elem != nil && !n.mixed {
		n = n.elem
	}
	if n.kind != "object" {
		return
	}
	pad := strings.Repeat("  ", level)
	if level > jsonShowDepth {
		b.WriteString(pad + "...\n")
		return
	}
	for i, k := range n.keys {
		if i == jsonMaxKeys {
			fmt.Fprintf(b, "%s... %d more keys\n", pad, len(n.keys)-jsonMaxKeys)
			return
		}
		fmt.Fprintf(b, "%s%s: %s\n", pad, k, jsonLabel(n.kids[i]))
		jsonEmit(b, n.kids[i], level+1)
	}
}
