// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// TokenUsage carries provider-reported token counts for one routed spawn.
// All fields are optional at the Event level (pointers + omitempty) so old
// JSONL lines without usage data keep parsing. When absent, the record counts
// no tokens.
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	// CachedTokens is cache-creation plus cache-read input (the harness totals).
	CachedTokens int64
	// CacheWriteTokens is the cache-creation part of CachedTokens, billed at
	// the model's cache-write price. Zero means the split is unknown and the
	// whole CachedTokens is billed at the cache-read price.
	CacheWriteTokens int64
}
