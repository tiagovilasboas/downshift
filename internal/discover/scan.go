// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// maxLineBytes caps one transcript line; a longer line is skipped.
	maxLineBytes = 8 << 20
	// maxPending bounds the calls waiting for their result per file.
	maxPending = 4096
)

// Harness ids this package reads.
const (
	HarnessClaudeCode = "claude-code"
	HarnessCodex      = "codex"
	HarnessAll        = "all"
)

// Options say what to scan. Home is injected so tests never touch a real one.
type Options struct {
	Home    string
	Harness string        // HarnessClaudeCode, HarnessCodex or HarnessAll
	Since   time.Duration // window ending now
	Label   string        // the window as the user wrote it, echoed in the report
	Top     int
	Now     time.Time
}

type source struct {
	harness string
	root    string
	parse   parser
}

// sources lists the transcript roots for the harness choice.
func sources(home, harness string) ([]source, error) {
	all := []source{
		{HarnessClaudeCode, filepath.Join(home, ".claude", "projects"), parseClaude},
		{HarnessCodex, filepath.Join(home, ".codex", "sessions"), parseCodex},
	}
	if harness == "" || harness == HarnessAll {
		return all, nil
	}
	for _, s := range all {
		if s.harness == harness {
			return []source{s}, nil
		}
	}
	return nil, fmt.Errorf("unknown harness %q (claude-code, codex or all)", harness)
}

// ParseSince reads a window such as 7d, 36h or 90m.
func ParseSince(s string) (time.Duration, error) {
	if n, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.Atoi(n)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("bad --since %q (use 7d, 36h, 90m)", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("bad --since %q (use 7d, 36h, 90m)", s)
	}
	return d, nil
}

// Scan streams every transcript in the window and returns the aggregate.
// A missing root is not an error: that harness simply has nothing to read.
func Scan(o Options) (Report, error) {
	srcs, err := sources(o.Home, o.Harness)
	if err != nil {
		return Report{}, err
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-o.Since)
	var tallies []*tally
	files := 0
	for _, s := range srcs {
		t := newTally(s.harness)
		tallies = append(tallies, t)
		_ = filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
				return nil
			}
			if info, err := d.Info(); err != nil || info.ModTime().Before(cutoff) {
				return nil
			}
			files++
			scanFile(p, s.parse, cutoff, t)
			return nil
		})
	}
	return merge(o.Label, files, tallies, o.Top), nil
}

// scanFile streams one transcript. Open and read errors end that file
// quietly: a transcript being written or removed must not fail the report.
func scanFile(path string, parse parser, cutoff time.Time, t *tally) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	pending := map[string][]Class{}
	r := bufio.NewReaderSize(f, 64<<10)
	var buf []byte
	for {
		line, over, err := readLine(r, buf, maxLineBytes)
		buf = line[:0]
		if !over && len(line) > 0 {
			if ev, ok := parse(line); ok && !(ev.at.Before(cutoff) && !ev.at.IsZero()) {
				apply(ev, pending, t)
			}
		}
		if err != nil {
			break
		}
	}
	for _, cs := range pending {
		for _, c := range cs {
			t.add(c, 0)
		}
	}
}

// apply classifies the calls of an event and pairs results with them.
func apply(ev event, pending map[string][]Class, t *tally) {
	for _, c := range ev.calls {
		if len(pending) >= maxPending {
			for id, cs := range pending {
				for _, k := range cs {
					t.add(k, 0)
				}
				delete(pending, id)
			}
		}
		pending[c.id] = append(pending[c.id], Classify(c.command))
	}
	for _, r := range ev.results {
		cs := pending[r.id]
		delete(pending, r.id)
		for _, c := range cs {
			t.add(c, r.bytes/int64(len(cs)))
		}
	}
}

// readLine reads one line into dst, keeping at most max bytes. over is true
// when the line was longer and was dropped. The error is io.EOF at the end.
func readLine(r *bufio.Reader, dst []byte, max int) (line []byte, over bool, err error) {
	dst = dst[:0]
	for {
		chunk, e := r.ReadSlice('\n')
		if !over {
			if len(dst)+len(chunk) > max {
				over, dst = true, dst[:0]
			} else {
				dst = append(dst, chunk...)
			}
		}
		if errors.Is(e, bufio.ErrBufferFull) {
			continue
		}
		return dst, over, e
	}
}
