// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The shape `downshift session --panel` prints (cmd/downshift/sessionpanel.go),
// trimmed to the fields the mod reads. Test data only.

export const PANEL_JSON = JSON.stringify({
  session: '034c74fd',
  summary: 'Downshift · 1 model',
  switched: 1,
  model_switched: 1,
  effort_changed: 0,
  held: 1,
  observed: 0,
  tokens: { spawns: 2, below_spawns: 1, billable: 30000, below_billable: 12000, rewritten_spawns: 1, rewritten_billable: 9000, fallback_spawns: 0, fallback_billable: 0, fallback_below_billable: 0, unobserved_spawns: 0, cache_read: 0 },
  tokens_line: '12.0k of 30.0k billable below the session model (1 of 2 spawns)',
  spawns: [
    { tool_use: 'k1', state: 'rewritten', from_model: 'claude-opus-5-5', to_model: 'claude-haiku-5-5', label: 'opus → haiku (small)' },
    { tool_use: 'k2', state: 'held', from_model: 'claude-sonnet-5-5', to_model: 'claude-sonnet-5-5', observed_model: 'claude-sonnet-5-5', label: 'held' },
  ],
  routing_why: [{ tool_use: 'k1', requested_model: 'claude-opus-5-5', final_model: 'claude-haiku-5-5', cell: 'TRIVIAL → small' }],
  compactor: {
    scope: 'last 30 days',
    commands: 494,
    compacted: 46,
    raw_bytes: 2837327,
    saved_bytes: 57630,
    estimated_tokens_saved: 14407,
    token_estimate: 'estimated: saved_bytes / 4',
    families: [
      { family: 'rg', commands: 12, compacted: 11, raw_tokens: 4700, avoided_tokens: 1960 },
      { family: 'git-grep', commands: 8, compacted: 6, raw_tokens: 2400, avoided_tokens: 800 },
      { family: 'go-test', commands: 5, compacted: 5, raw_tokens: 900, avoided_tokens: 120 },
    ],
    direct: {
      scope: 'last 30 days',
      commands: 3,
      saved_bytes: 4800,
      estimated_tokens_saved: 1200,
      line: 'direct calls, no session: 3, ~1.2k tokens saved',
    },
  },
  tokens_saved_estimate: 14407,
  tokens_saved_scope: 'last 30 days, all sessions',
  efficiency: {
    scope: { kind: 'days', days: 30 },
    harnesses: [
      {
        harness: 'claude-code',
        spawns: 5,
        spawns_by_tier: { small: { count: 2, pct: 40 }, mid: { count: 1, pct: 20 }, frontier: { count: 1, pct: 20 }, unknown: { count: 1, pct: 20 } },
        ended: 4,
        completed_tasks: 4,
        reworked: 1,
        rework_ratio: 0.25,
        completed_tier_known: 4,
        completed_non_frontier: 3,
        completed_tier_unknown: 0,
        premium_observed: true,
        premium_preserved_pct: 75,
      },
      {
        harness: 'codex',
        spawns: 1,
        spawns_by_tier: { small: { count: 0, pct: 0 }, mid: { count: 0, pct: 0 }, frontier: { count: 0, pct: 0 }, unknown: { count: 1, pct: 100 } },
        ended: 0,
        completed_tasks: 0,
        reworked: 0,
        rework_ratio: 0,
        completed_tier_known: 0,
        completed_non_frontier: 0,
        completed_tier_unknown: 0,
        premium_observed: false,
        premium_preserved_pct: 0,
      },
    ],
  },
  below_session_by_harness: [
    { harness: 'claude-code', observed: true, spawns: 2, below_spawns: 1, billable: 30000, below_billable: 12000, rewritten_spawns: 1, rewritten_billable: 9000, fallback_spawns: 0, fallback_billable: 0, fallback_below_billable: 0, unobserved_spawns: 0, cache_read: 0 },
    { harness: 'codex', observed: false, spawns: 0, below_spawns: 0, billable: 0, below_billable: 0, fallback_spawns: 1, fallback_billable: 4000, fallback_below_billable: 1000, unobserved_spawns: 0, cache_read: 0 },
  ],
})
