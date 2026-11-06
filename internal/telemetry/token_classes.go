// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// TokenClasses is a token count split the way providers bill it. Cache read is
// the cached total minus the cache-creation (write) part.
type TokenClasses struct {
	Records    int   `json:"records"`
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	// Total is Input + Output + CacheRead + CacheWrite.
	Total int64 `json:"total"`
}

// classesOf splits a usage into the token classes the reports count.
func classesOf(u TokenUsage) TokenClasses {
	cached := max(u.CachedTokens, 0)
	write := min(max(u.CacheWriteTokens, 0), cached)
	c := TokenClasses{
		Input:      max(u.InputTokens, 0),
		Output:     max(u.OutputTokens, 0),
		CacheRead:  cached - write,
		CacheWrite: write,
	}
	c.Total = c.Input + c.Output + c.CacheRead + c.CacheWrite
	return c
}

// Cache is cache read plus cache write.
func (c TokenClasses) Cache() int64 { return c.CacheRead + c.CacheWrite }

// Billable is input, output and cache write: the token definition of the
// seven-day comparison (docs/reference/beta-exit.md, pillar 3). Cache reads
// are left out because they follow the size of the context a child rereads,
// not the tier it ran on, so counting them buries the difference a tier makes.
func (c TokenClasses) Billable() int64 { return c.Input + c.Output + c.CacheWrite }

func (c *TokenClasses) add(o TokenClasses) {
	c.Records += o.Records
	c.Input += o.Input
	c.Output += o.Output
	c.CacheRead += o.CacheRead
	c.CacheWrite += o.CacheWrite
	c.Total += o.Total
}
