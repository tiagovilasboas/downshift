// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package hookutil

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO at transcript_path must not block the hook: Lstat rejects it before Open.
func TestLastInTranscriptTail_FIFODoesNotBlock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "t.jsonl")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	done := make(chan string, 1)
	go func() { done <- LastInTranscriptTail(p, func([]byte) string { return "x" }) }()
	select {
	case got := <-done:
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("LastInTranscriptTail blocked on a FIFO")
	}
}
