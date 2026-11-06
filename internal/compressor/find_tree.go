// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// findNode is one path component. A node is an input path when find printed
// it. Any other node is a directory the filter did not print, and it is shown
// with a trailing slash.
type findNode struct {
	name  string
	input bool
	kids  []*findNode
	index map[string]*findNode
}

// compressFind prints the paths as an indented tree: one component per line,
// indented by one space per level. A node that is not an input path ends in
// "/". The names without that slash are exactly the input paths, so the tree
// loses no path. It needs no danger gate because no line is dropped. It fails
// open on an empty line, an empty component (a double slash, a trailing slash
// or a bare "/"), a component that starts with a space (its indent would be
// ambiguous) and a duplicate path.
func compressFind(input []byte) ([]byte, bool, string) {
	root := &findNode{}
	seen := map[string]bool{}
	for _, p := range commandLines(input) {
		if seen[p] || !findPathOK(p) {
			return input, false, "find_unrecognized_structure_preserved"
		}
		seen[p] = true
		root.insert(p)
	}
	var b strings.Builder
	root.write(&b, 0)
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "find tree"
}

// findPathOK reports whether every component of p is a non-empty name that does
// not start with a space. The leading "/" of an absolute path is its root, not
// a component, so a bare "/" has no name and is refused.
func findPathOK(p string) bool {
	if p == "/" {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		if part == "" || part[0] == ' ' {
			return false
		}
	}
	return true
}

// insert adds path p below n: a "/" child first when p is absolute, then one
// child per component. The last node is the input path itself.
func (n *findNode) insert(p string) {
	cur := n
	if strings.HasPrefix(p, "/") {
		cur = cur.child("/")
		p = p[1:]
	}
	for _, part := range strings.Split(p, "/") {
		cur = cur.child(part)
	}
	cur.input = true
}

// child returns the child called name, creating it in first-seen order.
func (n *findNode) child(name string) *findNode {
	if c, ok := n.index[name]; ok {
		return c
	}
	if n.index == nil {
		n.index = map[string]*findNode{}
	}
	c := &findNode{name: name}
	n.index[name] = c
	n.kids = append(n.kids, c)
	return c
}

// write prints the children of n, each on its own line, indented by depth.
func (n *findNode) write(b *strings.Builder, depth int) {
	for _, k := range n.kids {
		b.WriteString(strings.Repeat(" ", depth))
		b.WriteString(k.label())
		b.WriteByte('\n')
		k.write(b, depth+1)
	}
}

// label is the printed name: the root "/" as it is, a directory the filter did
// not print with its slash, and an input path bare.
func (n *findNode) label() string {
	if n.input || n.name == "/" {
		return n.name
	}
	return n.name + "/"
}
