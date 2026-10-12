// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
)

// TranscriptTailBytes bounds how much of a session transcript is read.
// The latest session state sits at the end, and transcripts grow without bound.
const TranscriptTailBytes = 1 << 20

// LastInTranscriptTail scans the tail of a JSONL transcript and returns the
// last non-empty value pick extracts from a line. pick sees one line at a
// time and must return only an identifier (a model id), never content. The
// first line after a mid-file seek is partial; pick must tolerate bad JSON.
// Any problem returns "" (fail open).
func LastInTranscriptTail(path string, pick func(line []byte) string) string {
	if path == "" || filepath.Ext(path) != ".jsonl" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	if off := fi.Size() - TranscriptTailBytes; off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return ""
		}
	}
	var last string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), TranscriptTailBytes)
	for sc.Scan() {
		if v := pick(sc.Bytes()); v != "" {
			last = v
		}
	}
	return last
}
