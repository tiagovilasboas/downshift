// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Store limits and the clock are fields, so eviction is driven with small
// limits and a fake clock, with no sleeping and no 128 MiB write.

// When the total cap evicts the oldest entry, retrieving it fails with
// ErrNotFound (no bytes), and the newer entries still return their own bytes.
func TestEvictedByTotalCapIsNotFoundAndNewerSurvive(t *testing.T) {
	s, c := newStore(t)
	s.MaxEntryBytes = 100
	s.MaxTotalBytes = 250 // room for two 100-byte entries, not three
	body := func(ch string) []byte { return []byte(strings.Repeat(ch, 100)) }

	var ids []string
	for _, ch := range []string{"a", "b", "c"} {
		c.t = c.t.Add(time.Second)
		id, err := s.Put(body(ch))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	got, err := s.Get(ids[0])
	if !errors.Is(err, ErrNotFound) || len(got) != 0 {
		t.Fatalf("evicted entry: got %d bytes, err %v; want ErrNotFound and no bytes", len(got), err)
	}
	for i, ch := range []string{"b", "c"} {
		got, err := s.Get(ids[i+1])
		if err != nil || string(got) != string(body(ch)) {
			t.Fatalf("surviving entry %d: err %v, bytes %q", i+1, err, got)
		}
	}
}

// An entry past the TTL is dropped by a later Put's prune: retrieving it is
// ErrNotFound (never another entry's bytes) and the fresh entry is intact.
func TestExpiredEntryPrunedByLaterPutIsNotFound(t *testing.T) {
	s, c := newStore(t)
	old, err := s.Put([]byte("old output\n"))
	if err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(s.TTL + time.Minute)
	fresh, err := s.Put([]byte("fresh output\n"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(old)
	if !errors.Is(err, ErrNotFound) || len(got) != 0 {
		t.Fatalf("pruned entry: got %q, err %v; want ErrNotFound and no bytes", got, err)
	}
	got, err = s.Get(fresh)
	if err != nil || string(got) != "fresh output\n" {
		t.Fatalf("fresh entry: %q, %v", got, err)
	}
}

// An expired entry that no Put has pruned yet is refused with ErrExpired, no
// bytes, and removed; asking again then reports ErrNotFound.
func TestExpiredEntryOnGetIsExpiredThenNotFound(t *testing.T) {
	s, c := newStore(t)
	id, _ := s.Put([]byte("keep me\n"))
	c.t = c.t.Add(s.TTL + time.Second)

	got, err := s.Get(id)
	if !errors.Is(err, ErrExpired) || len(got) != 0 {
		t.Fatalf("first get: %q, %v", got, err)
	}
	got, err = s.Get(id)
	if !errors.Is(err, ErrNotFound) || len(got) != 0 {
		t.Fatalf("second get: %q, %v", got, err)
	}
}
