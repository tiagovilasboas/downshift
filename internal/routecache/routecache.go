// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package routecache is a same-process memo for routing decisions. A repeated
// (prompt, harness, current model) triple reuses the stored Decision instead
// of re-running classification, keeping repeated work out of long-lived
// processes (daemons, servers, batch benchmark runs).
//
// Deliberately absent: cross-process hook caching. Each hook invocation runs
// in its own short-lived process, so a file-backed cache would buy nothing
// for sub-millisecond classification while adding locking, invalidation, and
// a prompt store — and prompts are never persisted by design. The hook path
// stays stateless; long-lived embeds opt in here.
//
// The cache stores Decisions, never prompts beyond the in-memory key the
// caller already holds. A nil *Cache is valid and disables memoization.
package routecache

import (
	"sync"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// Key scopes a cached decision. Prompts are held in memory only and never
// written anywhere by this package.
type Key struct {
	Prompt       string
	Harness      string
	CurrentModel string
}

// Cache is a bounded FIFO memo of routing decisions. The zero value is not
// usable; build one with New. Safe for concurrent use.
type Cache struct {
	mu    sync.RWMutex
	max   int
	order []Key
	items map[Key]core.Decision
}

// New returns a Cache holding up to max decisions. Non-positive max means
// unbounded; callers that want that should still prefer a small bound.
func New(max int) *Cache {
	return &Cache{max: max, items: make(map[Key]core.Decision)}
}

// Get returns the stored decision and true on a hit. A nil cache never hits.
func (c *Cache) Get(k Key) (core.Decision, bool) {
	if c == nil {
		return core.Decision{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.items[k]
	return d, ok
}

// Put stores d under k, evicting the oldest entry when the bound is reached.
// A nil cache discards the write.
func (c *Cache) Put(k Key, d core.Decision) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[k]; !exists {
		if c.max > 0 && len(c.items) >= c.max {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.items, oldest)
		}
		c.order = append(c.order, k)
	}
	c.items[k] = d
}

// Len returns the number of stored decisions. A nil cache holds zero.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
