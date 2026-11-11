// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Bounds of the transcript read behind the post-run honor check. The most
// recent files are read first, and only the tail of each: the rows being
// checked are recent.
const (
	maxToolResultFiles     = 60
	maxToolResultFileBytes = 8 << 20
)

// readLimits bounds one read of the transcript directory.
type readLimits struct {
	maxFiles     int
	maxFileBytes int64
}

// ReadToolResults is Claude Code's hookport.ReadToolResultsFunc: it looks for
// ids in the tool results the model received, as the session transcripts under
// ~/.claude/projects record them. Offline, in memory, bounded; it keeps no
// text and no path, and any failure reports ok false.
func ReadToolResults(ids []string) ([]hookport.ToolResultSample, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, false
	}
	return readToolResults(filepath.Join(home, ".claude", "projects"), ids,
		readLimits{maxFiles: maxToolResultFiles, maxFileBytes: maxToolResultFileBytes})
}

type transcriptFile struct {
	path  string
	mtime int64
}

func readToolResults(root string, ids []string, lim readLimits) ([]hookport.ToolResultSample, bool) {
	if len(ids) == 0 {
		return nil, false
	}
	var files []transcriptFile
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() || filepath.Ext(p) != ".jsonl" {
			return nil // a file we cannot see is a file we skip
		}
		if info, ierr := d.Info(); ierr == nil {
			files = append(files, transcriptFile{p, info.ModTime().UnixNano()})
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mtime > files[j].mtime })
	if len(files) > lim.maxFiles {
		files = files[:lim.maxFiles]
	}
	var out []hookport.ToolResultSample
	readAny := false
	for _, f := range files {
		samples, ok := scanTranscriptTail(f.path, ids, lim.maxFileBytes)
		if ok {
			readAny = true
			out = append(out, samples...)
		}
	}
	if !readAny {
		return nil, false
	}
	return out, true
}

// scanTranscriptTail reads at most max bytes from the end of one transcript
// and returns a sample for every user tool_result that names one of ids.
func scanTranscriptTail(path string, ids []string, max int64) ([]hookport.ToolResultSample, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return nil, false
	}
	rd := bufio.NewReaderSize(f, 1<<20)
	if fi.Size() > max {
		if _, err := f.Seek(fi.Size()-max, io.SeekStart); err != nil {
			return nil, false
		}
		rd.Reset(f)
		_, _ = rd.ReadBytes('\n') // drop the partial first line
	}
	var out []hookport.ToolResultSample
	for {
		line, err := rd.ReadBytes('\n')
		out = append(out, toolResultSamples(line, ids)...)
		if err != nil {
			break
		}
	}
	return out, true
}

// toolResultSamples parses one transcript line. Malformed lines, lines of
// another type and results that name no id give nothing.
func toolResultSamples(line []byte, ids []string) []hookport.ToolResultSample {
	if !bytes.Contains(line, []byte(`"tool_result"`)) || !containsAnyID(line, ids) {
		return nil
	}
	var row struct {
		Type    string `json:"type"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &row) != nil || row.Type != "user" {
		return nil
	}
	var blocks []struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(row.Message.Content, &blocks) != nil {
		return nil
	}
	var out []hookport.ToolResultSample
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		text := resultText(b.Content)
		for _, id := range ids {
			if strings.Contains(text, id) {
				out = append(out, hookport.ToolResultSample{ID: id, Bytes: len(text)})
			}
		}
	}
	return out
}

func containsAnyID(line []byte, ids []string) bool {
	for _, id := range ids {
		if bytes.Contains(line, []byte(id)) {
			return true
		}
	}
	return false
}

// resultText is the text of a tool_result's content: a string, or the text
// blocks of a list, joined as they are.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}
