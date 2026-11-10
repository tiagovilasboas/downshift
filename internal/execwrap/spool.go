// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

// SpoolLimit bounds the stdout one run may spool to disk past the hold limit.
// Past it the spooled bytes are written out raw and the rest streams (reason
// too_large). It matches the CCR store's per-entry cap (ccr.Default).
var SpoolLimit int64 = 32 << 20

// What a spooled run shows: its first spoolHeadBytes and its last
// spoolTailBytes, each cut to whole lines when it has any.
const (
	spoolHeadBytes = 24 << 10
	spoolTailBytes = 8 << 10
)

// spool holds the stdout of a run past the hold limit: every byte goes to a
// private temporary file, and only a bounded head and tail stay in memory.
type spool struct {
	f       *os.File
	head    []byte
	tail    []byte // the last bytes written; trimmed to spoolTailBytes as it grows
	n       int64  // bytes in the file
	pending []byte // bytes a failed write could not put in the file
}

// newSpool creates the spool file in the OS temporary directory (0600).
func newSpool() (*spool, error) {
	f, err := os.CreateTemp("", "downshift-spool-*")
	if err != nil {
		return nil, err
	}
	return &spool{f: f}, nil
}

// write appends p to the file and to the in-memory head and tail. On a write
// error the unwritten bytes are kept in pending, so a replay loses none.
func (s *spool) write(p []byte) error {
	if room := spoolHeadBytes - len(s.head); room > 0 {
		s.head = append(s.head, p[:min(room, len(p))]...)
	}
	if len(p) >= spoolTailBytes {
		s.tail = append(make([]byte, 0, 2*spoolTailBytes), p[len(p)-spoolTailBytes:]...)
	} else {
		s.tail = append(s.tail, p...)
		if len(s.tail) > 2*spoolTailBytes {
			s.tail = append(s.tail[:0], s.tail[len(s.tail)-spoolTailBytes:]...)
		}
	}
	if len(s.pending) > 0 {
		s.pending = append(s.pending, p...)
		return io.ErrShortWrite
	}
	n, err := s.f.Write(p)
	s.n += int64(n)
	if err != nil {
		s.pending = append(s.pending, p[n:]...)
	}
	return err
}

// replayTo writes every spooled byte to w, in order.
func (s *spool) replayTo(w io.Writer) {
	if _, err := s.f.Seek(0, io.SeekStart); err == nil {
		_, _ = io.Copy(w, io.LimitReader(s.f, s.n))
	}
	_, _ = w.Write(s.pending)
}

// readAll reads the spooled bytes back once, for the keep port.
func (s *spool) readAll() ([]byte, error) {
	if len(s.pending) > 0 {
		return nil, io.ErrShortWrite
	}
	if _, err := s.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data := make([]byte, s.n)
	_, err := io.ReadFull(s.f, data)
	return data, err
}

// discard closes and removes the spool file.
func (s *spool) discard() {
	_ = s.f.Close()
	_ = os.Remove(s.f.Name())
}

// view is what a spooled run prints once its raw output is kept under id: the
// head, a line naming how many bytes were cut and where to read them, the
// tail and the marker.
func (s *spool) view(id string) []byte {
	head := headLines(s.head)
	tail := s.tail
	if len(tail) > spoolTailBytes {
		tail = tail[len(tail)-spoolTailBytes:]
	}
	tail = tailLines(tail)
	cut := s.n - int64(len(head)) - int64(len(tail))
	var b bytes.Buffer
	b.Write(head)
	if len(head) > 0 && head[len(head)-1] != '\n' {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "… %d bytes cut (downshift retrieve %s)\n", cut, id)
	b.Write(tail)
	if len(tail) > 0 && tail[len(tail)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString(MarkerWithID(int(s.n), b.Len(), id))
	return b.Bytes()
}

// headLines cuts head after its last newline, or, with none, on a character
// boundary.
func headLines(head []byte) []byte {
	if i := bytes.LastIndexByte(head, '\n'); i >= 0 {
		return head[:i+1]
	}
	i := len(head) - 1
	for i > 0 && len(head)-i < utf8.UTFMax && !utf8.RuneStart(head[i]) {
		i--
	}
	if i >= 0 && !utf8.FullRune(head[i:]) {
		return head[:i] // never end on half a character
	}
	return head
}

// tailLines cuts tail after its first newline, or, with none, on a character
// boundary.
func tailLines(tail []byte) []byte {
	if i := bytes.IndexByte(tail, '\n'); i >= 0 && i < len(tail)-1 {
		return tail[i+1:]
	}
	i := 0
	for i < len(tail) && !utf8.RuneStart(tail[i]) {
		i++
	}
	return tail[i:]
}
