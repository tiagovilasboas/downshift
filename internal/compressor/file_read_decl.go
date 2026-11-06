// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Declaration heuristics for the folded view of a source file. They read the
// start of a line only, with no language grammar, so one set serves Go, Python,
// JS/TS, Rust, Java, Kotlin, C-family, Ruby and PHP alike. A miss folds a line
// the reader can fetch; a false hit only keeps one more line.
var (
	// reStrongDecl: a keyword that always opens a named declaration. Allowed up
	// to declIndentMax columns deep, so methods inside a class or impl count.
	reStrongDecl = regexp.MustCompile(`^(?:(?:export|pub(?:\([a-z]+\))?|public|private|protected|internal|static|async|abstract|final|default|open|override|unsafe|extern|declare|data|sealed|suspend|inline)\s+)*(?:func|fn|def|class|struct|enum|trait|interface|impl|type|function|fun|module|namespace|object|package|protocol|extension|mod|record)\b`)
	// reWeakDecl: a keyword that also opens local variables, so only at column 0.
	reWeakDecl = regexp.MustCompile(`^(?:(?:export|pub|public|private|static|final|default|declare)\s+)*(?:const|let|var|val|static)\b`)
	reHeading  = regexp.MustCompile(`^#{1,6}\s+\S`)
)

// declIndentMax is the deepest indent, in columns (a tab counts 4), at which a
// strong keyword still marks a declaration.
const declIndentMax = 8

// sourceExts are the file extensions whose text the folded view may read.
// Anything else (data, config, logs, lockfiles, unknown) stays raw: its
// meaning is in the lines a declaration scan would fold.
var sourceExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".rs": true, ".java": true, ".kt": true, ".kts": true,
	".scala": true, ".rb": true, ".php": true, ".c": true, ".h": true, ".cc": true,
	".cpp": true, ".hpp": true, ".cs": true, ".swift": true, ".lua": true,
	".ex": true, ".exs": true, ".dart": true, ".zig": true, ".vue": true, ".svelte": true,
	".md": true, ".mdx": true, ".markdown": true,
}

// fileExt is the lower-case extension of a display name, which may be quoted.
func fileExt(name string) string {
	return strings.ToLower(filepath.Ext(strings.Trim(name, `'"`)))
}

func isMarkdown(ext string) bool { return ext == ".md" || ext == ".mdx" || ext == ".markdown" }

// isDeclLine reports whether line opens a declaration (or, in Markdown, a heading).
func isDeclLine(line string, markdown bool) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return false
	}
	indent := 0
	for _, c := range line[:len(line)-len(trimmed)] {
		if c == '\t' {
			indent += 4
		} else {
			indent++
		}
	}
	switch {
	case markdown:
		return indent == 0 && reHeading.MatchString(trimmed)
	case indent == 0 && reWeakDecl.MatchString(trimmed):
		return true
	}
	return indent <= declIndentMax && reStrongDecl.MatchString(trimmed)
}
