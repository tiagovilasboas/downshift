// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStore(t *testing.T) (Store, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return Store{Dir: filepath.Join(t.TempDir(), "ccr"), TTL: time.Hour, MaxEntryBytes: 1024, MaxTotalBytes: 1 << 20, Now: c.now}, c
}

func TestPutGetRoundTrip(t *testing.T) {
	s, _ := newStore(t)
	id, err := s.Put([]byte("a\nb\n"))
	if err != nil || !ValidID(id) {
		t.Fatalf("id %q err %v", id, err)
	}
	got, err := s.Get(id)
	if err != nil || string(got) != "a\nb\n" {
		t.Fatalf("got %q err %v", got, err)
	}
	if info, _ := os.Stat(filepath.Join(s.Dir, id+".txt")); info.Mode().Perm()&0o077 != 0 && filepath.Separator == '/' {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestSameContentSameID(t *testing.T) {
	s, _ := newStore(t)
	a, _ := s.Put([]byte("same"))
	b, _ := s.Put([]byte("same"))
	c, _ := s.Put([]byte("other"))
	if a != b || a == c {
		t.Fatalf("ids %s %s %s", a, b, c)
	}
}

func TestTTLExpiry(t *testing.T) {
	s, c := newStore(t)
	id, _ := s.Put([]byte("x"))
	c.t = c.t.Add(30 * time.Minute)
	if _, err := s.Get(id); err != nil {
		t.Fatalf("fresh: %v", err)
	}
	c.t = c.t.Add(2 * time.Hour)
	if _, err := s.Get(id); !errors.Is(err, ErrExpired) {
		t.Fatalf("want expired, got %v", err)
	}
	if _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want removed, got %v", err)
	}
}

func TestPutRefreshesMtime(t *testing.T) {
	s, c := newStore(t)
	id, _ := s.Put([]byte("x"))
	c.t = c.t.Add(50 * time.Minute)
	if _, err := s.Put([]byte("x")); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(50 * time.Minute)
	if _, err := s.Get(id); err != nil {
		t.Fatalf("refresh lost: %v", err)
	}
}

func TestPrunesExpiredOnPut(t *testing.T) {
	s, c := newStore(t)
	old, _ := s.Put([]byte("old"))
	c.t = c.t.Add(2 * time.Hour)
	if _, err := s.Put([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, old+".txt")); !os.IsNotExist(err) {
		t.Fatalf("expired entry kept: %v", err)
	}
}

func TestMaxTotalPrunesOldestNeverNew(t *testing.T) {
	s, c := newStore(t)
	s.MaxTotalBytes = 25
	var ids []string
	for _, body := range []string{strings.Repeat("a", 10), strings.Repeat("b", 10), strings.Repeat("c", 10)} {
		c.t = c.t.Add(time.Minute)
		id, err := s.Put([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if _, err := s.Get(ids[0]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("oldest kept: %v", err)
	}
	for _, id := range ids[1:] {
		if _, err := s.Get(id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	s.MaxTotalBytes = 1
	c.t = c.t.Add(time.Minute)
	id, err := s.Put([]byte(strings.Repeat("d", 10)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(id); err != nil {
		t.Fatalf("new id pruned: %v", err)
	}
}

func TestMaxEntryRefused(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.Put(make([]byte, 1025)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Put(make([]byte, 1024)); err != nil {
		t.Fatalf("at limit: %v", err)
	}
}

func TestInvalidIDs(t *testing.T) {
	s, _ := newStore(t)
	for _, id := range []string{"../x", "ABCDEF012345", "abc", "0123456789abc", "", "0123456789a/"} {
		if _, err := s.Get(id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("%q: got %v", id, err)
		}
	}
	if _, err := s.Get("0123456789ab"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestSecretLinesRedacted(t *testing.T) {
	s, _ := newStore(t)
	id, _ := s.Put([]byte("ok line\nAPI_KEY=abc123\nBearer xyz\nfine\n"))
	got, _ := s.Get(id)
	want := "ok line\n" + redactedLine + "\n" + redactedLine + "\nfine\n"
	if string(got) != want {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(string(got), "abc123") {
		t.Fatal("secret leaked")
	}
}
