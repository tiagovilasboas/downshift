// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// backupEdgeBytes bounds how much of a backup is read to find its last line.
// The first line is read through a buffer of the same size.
const backupEdgeBytes = 64 * 1024

// backupLineLimit is how many backups the stats line names.
const backupLineLimit = 3

// BackupDetail describes one rotated or manual backup beside the event log.
// It carries no event content: only the file name, its size and, when both
// edge lines parse, the timestamps of its first and last event.
type BackupDetail struct {
	Name    string
	Size    int64
	HasSpan bool
	First   time.Time
	Last    time.Time
}

// PrintBackupLine writes one line naming up to three backups beside the
// default event log, with size and date span. It writes nothing when no
// backup exists. Only file names appear, never full paths.
func PrintBackupLine(w io.Writer) {
	shown, total := describeBackups(defaultEventPath(), backupLineLimit)
	if total == 0 {
		return
	}
	parts := make([]string, 0, len(shown))
	for _, b := range shown {
		parts = append(parts, formatBackup(b))
	}
	line := "Backups beside the log: " + strings.Join(parts, "; ")
	if total > len(shown) {
		line += fmt.Sprintf("; and %d more", total-len(shown))
	}
	fmt.Fprintln(w, line+".")
}

// describeBackups returns details for up to limit backups of logPath, in
// listing order, and the total number of backups found.
func describeBackups(logPath string, limit int) ([]BackupDetail, int) {
	entries, err := listBackups(logPath)
	if err != nil {
		return nil, 0
	}
	dir := filepath.Dir(logPath)
	var shown []BackupDetail
	for _, e := range entries {
		if len(shown) == limit {
			break
		}
		full := filepath.Join(dir, e.Name())
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		d := BackupDetail{Name: e.Name(), Size: info.Size()}
		d.First, d.Last, d.HasSpan = readEdgeTimestamps(full, info.Size())
		shown = append(shown, d)
	}
	return shown, len(entries)
}

func formatBackup(b BackupDetail) string {
	s := fmt.Sprintf("%s (%d bytes", b.Name, b.Size)
	if b.HasSpan {
		s += fmt.Sprintf(", %s to %s", b.First.Format("2006-01-02"), b.Last.Format("2006-01-02"))
	}
	return s + ")"
}

// readEdgeTimestamps reads the first and last line of an event log and
// returns their timestamps. It is fail-silent: any unreadable or malformed
// edge line yields HasSpan false.
func readEdgeTimestamps(path string, size int64) (first, last time.Time, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return first, last, false
	}
	defer f.Close()

	head, err := bufio.NewReaderSize(f, backupEdgeBytes).ReadSlice('\n')
	if err != nil && err != io.EOF {
		return first, last, false
	}
	first, ok = parseLineTimestamp(head)
	if !ok {
		return first, last, false
	}

	offset := size - backupEdgeBytes
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return first, last, false
	}
	tail, err := io.ReadAll(io.LimitReader(f, backupEdgeBytes))
	if err != nil {
		return first, last, false
	}
	tail = bytes.TrimRight(tail, "\r\n")
	cut := bytes.LastIndexByte(tail, '\n')
	if cut < 0 && offset > 0 {
		// The window starts inside a line, so the last line is not whole.
		return first, last, false
	}
	last, ok = parseLineTimestamp(tail[cut+1:])
	return first, last, ok
}

// parseLineTimestamp decodes only the timestamp field of one event line.
// Other fields are never retained.
func parseLineTimestamp(line []byte) (time.Time, bool) {
	var ev struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &ev); err != nil || ev.Timestamp == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
