// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Caps that keep the accumulator small whatever a transcript holds.
const (
	maxResultsSeen  = 20000 // distinct tool results counted
	maxLengthsPerID = 64    // distinct result lengths kept per id
)

// retrieveHint is the text a compacted output carries to name its kept id:
// `downshift retrieve <id>` or `downshift retrieve --session=<ref> <id>`. It
// is the only thing matched; the prose around it may change.
const retrieveHint = "downshift retrieve "

// resultAccumulator collects, per wanted id and result length, how many
// distinct tool results named the id. A result is identified by its
// tool_use_id, so a transcript that repeats another's history counts once.
// Only counts are kept, never text.
type resultAccumulator struct {
	want      map[string]struct{}
	counts    map[string]map[int]int
	seen      map[string]struct{}
	truncated bool // a cap above dropped something
}

func newResultAccumulator(ids []string) *resultAccumulator {
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	return &resultAccumulator{want: want, counts: map[string]map[int]int{}, seen: map[string]struct{}{}}
}

// fileScan is what reading one transcript showed.
type fileScan struct {
	ok         bool      // the file was readable
	cut        bool      // only its tail was read
	read       int64     // bytes read
	firstStamp time.Time // timestamp of the first line read that has one
}

// scanFile reads at most max bytes from the end of one transcript.
func (a *resultAccumulator) scanFile(path string, max int64) fileScan {
	f, err := os.Open(path)
	if err != nil {
		return fileScan{}
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || max <= 0 {
		return fileScan{}
	}
	out := fileScan{ok: true, read: min(fi.Size(), max)}
	rd := bufio.NewReaderSize(f, 1<<20)
	if fi.Size() > max {
		if _, err := f.Seek(fi.Size()-max, io.SeekStart); err != nil {
			return fileScan{}
		}
		rd.Reset(f)
		_, _ = rd.ReadString('\n') // drop the partial first line
		out.cut = true
	}
	for {
		line, err := rd.ReadString('\n')
		if out.firstStamp.IsZero() {
			out.firstStamp = lineStamp(line)
		}
		a.addLine(line)
		if err != nil {
			break
		}
	}
	return out
}

// lineStamp reads the "timestamp" field of a transcript line, or the zero time.
func lineStamp(line string) time.Time {
	const key = `"timestamp":"`
	i := strings.Index(line, key)
	if i < 0 {
		return time.Time{}
	}
	rest := line[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, rest[:j])
	if err != nil {
		return time.Time{}
	}
	return t
}

// addLine counts the user tool results of one transcript line that carry a
// retrieve hint for a wanted id. Malformed lines, other types and results
// without such a hint add nothing. The hint test runs on the raw line first,
// in one pass, so most lines are never parsed.
func (a *resultAccumulator) addLine(line string) {
	if !a.hasWantedHint(line) {
		return
	}
	var row struct {
		Type    string `json:"type"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal([]byte(line), &row) != nil || row.Type != "user" {
		return
	}
	var blocks []struct {
		Type      string          `json:"type"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
	}
	if json.Unmarshal(row.Message.Content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		if b.Type == "tool_result" {
			a.addResult(b.ToolUseID, resultText(b.Content))
		}
	}
}

func (a *resultAccumulator) hasWantedHint(s string) bool {
	return forEachHint(s, a.want, func(string) bool { return true })
}

// addResult counts one tool result's text once per id it names.
func (a *resultAccumulator) addResult(useID, text string) {
	var ids []string
	forEachHint(text, a.want, func(id string) bool {
		for _, have := range ids {
			if have == id {
				return false
			}
		}
		ids = append(ids, id)
		return false
	})
	if len(ids) == 0 {
		return
	}
	if useID != "" {
		if _, dup := a.seen[useID]; dup {
			return
		}
		if len(a.seen) >= maxResultsSeen {
			a.truncated = true
			return
		}
		a.seen[useID] = struct{}{}
	}
	for _, id := range ids {
		byLen := a.counts[id]
		if byLen == nil {
			byLen = map[int]int{}
			a.counts[id] = byLen
		}
		if _, ok := byLen[len(text)]; !ok && len(byLen) >= maxLengthsPerID {
			a.truncated = true
			continue
		}
		byLen[len(text)]++
	}
}

// samples lists what was counted, in id then length order.
func (a *resultAccumulator) samples() []hookport.ToolResultSample {
	var out []hookport.ToolResultSample
	for id, byLen := range a.counts {
		for n, c := range byLen {
			out = append(out, hookport.ToolResultSample{ID: id, Bytes: n, Count: c})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Bytes < out[j].Bytes
	})
	return out
}

// resultText is the text of a tool_result's content: a string, or the text
// blocks of a list, joined as they are. Image blocks add nothing.
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

// forEachHint walks the retrieve hints in s, in one pass, and calls yield with
// each hint's id that is in want. It stops, returning true, when yield does.
// A hint is `downshift retrieve ` plus an optional `--session=<ref> ` and
// exactly 12 lowercase hex characters that are not the start of a longer hex
// run.
func forEachHint(s string, want map[string]struct{}, yield func(id string) bool) bool {
	for {
		i := strings.Index(s, retrieveHint)
		if i < 0 {
			return false
		}
		s = s[i+len(retrieveHint):]
		rest := s
		if strings.HasPrefix(rest, "--session=") {
			j := strings.IndexByte(rest, ' ')
			if j < 0 || j > 64 {
				continue
			}
			rest = rest[j+1:]
		}
		if len(rest) < 12 || !isLowerHex(rest[:12]) || (len(rest) > 12 && isLowerHex(rest[12:13])) {
			continue
		}
		if _, ok := want[rest[:12]]; ok && yield(rest[:12]) {
			return true
		}
	}
}

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
