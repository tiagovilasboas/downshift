// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHoldStateLock(t *testing.T) {
	if os.Getenv("DOWNSHIFT_HOLD_LOCK") != "1" {
		t.Skip()
	}
	f, err := os.OpenFile(os.Getenv("DOWNSHIFT_LOCK_PATH"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		t.Fatal(err)
	}
	defer unlockFile(f)
	_, _ = os.Stdout.WriteString("held\n")
	_ = os.Stdout.Sync()
	time.Sleep(3 * time.Second)
}

func TestRetainCompaction_DropsAgedAndExcess(t *testing.T) {
	previous := compactionMaxRecords
	compactionMaxRecords = 2
	t.Cleanup(func() { compactionMaxRecords = previous })

	dir := t.TempDir()
	path := filepath.Join(dir, "context-compactions.jsonl")
	old := CompactionRecord{
		Timestamp:   time.Now().UTC().Add(-compactionMaxAge - time.Hour).Format(time.RFC3339Nano),
		Harness:     "claude-code",
		OutputBytes: 9,
	}
	if err := AppendCompaction(path, old); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"output_bytes":9`) {
		t.Fatalf("aged record kept: %s", raw)
	}

	for _, n := range []int64{1, 2, 3} {
		rec := CompactionRecord{
			Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
			Harness:     "claude-code",
			OutputBytes: n,
		}
		if err := AppendCompaction(path, rec); err != nil {
			t.Fatal(err)
		}
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, `"output_bytes":1`) || !strings.Contains(text, `"output_bytes":2`) || !strings.Contains(text, `"output_bytes":3`) {
		t.Fatalf("retention kept the wrong tail: %s", text)
	}
}
