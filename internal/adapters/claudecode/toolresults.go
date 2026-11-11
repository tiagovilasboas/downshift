// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Bounds of the transcript read behind the post-run honor check. The most
// recent files are read first, and only the tail of each: the rows being
// checked are recent. Whatever a bound leaves out is reported as truncation
// and as the start of the coverage, so the shared check calls those rows
// unverified.
const (
	maxToolResultFiles      = 60
	maxToolResultFileBytes  = 8 << 20
	maxToolResultTotalBytes = 64 << 20
	toolResultTimeout       = 3 * time.Second
)

// readLimits bounds one read of the transcript directory.
type readLimits struct {
	maxFiles      int
	maxFileBytes  int64
	maxTotalBytes int64
	// timeout is checked before each file, not inside one: a file is at most
	// maxFileBytes, so the read overshoots by one file at most.
	timeout time.Duration
	now     func() time.Time // nil means time.Now
}

// ReadToolResults is Claude Code's hookport.ReadToolResultsFunc: it looks for
// ids in the tool results the model received, as the session transcripts under
// ~/.claude/projects record them. Offline, in memory, bounded; it keeps no
// text and no path, and any failure reports an unreadable result.
//
// Known limits, all conservative: a result the harness spilled to a file, an
// image block, or text that is not valid UTF-8 can make a row read as
// mismatch or not found though the model got it.
func ReadToolResults(ids []string) hookport.ToolResults {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return hookport.ToolResults{}
	}
	return readToolResults(filepath.Join(home, ".claude", "projects"), ids, readLimits{
		maxFiles:      maxToolResultFiles,
		maxFileBytes:  maxToolResultFileBytes,
		maxTotalBytes: maxToolResultTotalBytes,
		timeout:       toolResultTimeout,
	})
}

type transcriptFile struct {
	path  string
	mtime time.Time
}

// readToolResults scans the transcripts under root, newest first. Coverage:
// Since is the later of the oldest line read and the newest moment a skipped
// or cut-off part may hold (a skipped file's mtime, a tail-cut file's first
// line read). A row before Since is outside what the read saw.
func readToolResults(root string, ids []string, lim readLimits) hookport.ToolResults {
	if len(ids) == 0 {
		return hookport.ToolResults{}
	}
	now := lim.now
	if now == nil {
		now = time.Now
	}
	deadline := now().Add(lim.timeout)
	files := listTranscripts(root)
	acc := newResultAccumulator(ids)
	budget := lim.maxTotalBytes
	var res hookport.ToolResults
	var oldest, gap time.Time // oldest line read; newest moment left unread
	for i, f := range files {
		if i >= lim.maxFiles || budget <= 0 || !now().Before(deadline) {
			res.Truncated = true
			gap = laterOf(gap, f.mtime) // files are newest first
			break
		}
		scan := acc.scanFile(f.path, min(lim.maxFileBytes, budget))
		if !scan.ok { // unreadable: what it holds is unseen
			res.Truncated = true
			gap = laterOf(gap, f.mtime)
			continue
		}
		res.Readable = true
		budget -= scan.read
		first := scan.firstStamp
		if first.IsZero() {
			first = f.mtime
		}
		if oldest.IsZero() || first.Before(oldest) {
			oldest = first
		}
		if scan.cut {
			res.Truncated = true
			gap = laterOf(gap, first)
		}
	}
	if !res.Readable {
		return hookport.ToolResults{}
	}
	res.Truncated = res.Truncated || acc.truncated
	res.Since = laterOf(oldest, gap)
	res.Samples = acc.samples()
	return res
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// listTranscripts returns the regular .jsonl files under root, newest first.
// A file it cannot see is skipped.
func listTranscripts(root string) []transcriptFile {
	var files []transcriptFile
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() || filepath.Ext(p) != ".jsonl" {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			files = append(files, transcriptFile{p, info.ModTime()})
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.After(files[j].mtime) })
	return files
}
