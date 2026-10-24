// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Sample `session --panel` answers for the parser and client tests. Shapes follow
// the Go structs (`telemetry.PanelView`, `accounting.Numbers`). Synthetic values,
// no real session data.

const HASH_A = 'a'.repeat(64)
const HASH_B = 'b'.repeat(64)

/** Main today: session view plus the report block, no new fields. */
export const TODAY_PANEL_JSON = JSON.stringify({
  session: '3f9a1c2e',
  switched: 1,
  model_switched: 1,
  effort_changed: 0,
  held: 1,
  held_guardrail: 0,
  upshift_not_applied: 0,
  observed: 0,
  tokens: { work_saved: 48000, cache_saved: 0 },
  tokens_line: '48k tokens saved',
  by_agent_type: [],
  spawns: [
    {
      tool_use: HASH_A,
      state: 'switched',
      from_model: 'opus',
      to_model: 'sonnet',
      label: 'opus → sonnet (mid)',
      tokens: { work_saved: 48000, cache_saved: 0 },
      tokens_label: '48k tokens · saved from opus',
    },
    {
      tool_use: HASH_B,
      state: 'held',
      from_model: 'opus',
      to_model: 'opus',
      label: 'held: original model kept',
      tokens: { work_saved: 0, cache_saved: 0 },
      tokens_label: '',
    },
  ],
  report: {
    scope: { kind: 'days', days: 30 },
    routing: { work_saved: 48000, cache_saved: 0 },
    compactor: { commands: 12, compacted: 9, raw_bytes: 90000, saved_bytes: 60000, tokens: 15000 },
    model_switched: 1,
    effort_changed: 0,
    held: 1,
    held_guardrail: 0,
    upshift_not_applied: 0,
    tokens_saved_estimate: 63000,
    display: {
      tokens_saved: '63000 tokens',
      routing: '48000 work tokens',
      compactor: '15000 tokens (estimate)',
    },
  },
  compactor: { scope: 'last 30 days', commands: 12, compacted: 9, estimated_tokens_saved: 15000 },
  tokens_saved_estimate: 63000,
})

/** With the fields G1a, G1b and G2-era binaries add, plus an unknown field. */
export const PANEL_WITH_NEW_FIELDS_JSON = JSON.stringify({
  session: '3f9a1c2e',
  switched: 2,
  model_switched: 1,
  effort_changed: 1,
  held: 0,
  summary: 'Downshift · 1 model · 1 effort',
  future_field: { anything: true },
  spawns: [
    {
      tool_use: HASH_A,
      state: 'switched',
      from_model: 'opus',
      to_model: 'sonnet',
      label: 'opus → sonnet (mid)',
      saved_total: 58000,
      saved_short: '58k',
    },
    {
      tool_use: HASH_B,
      state: 'switched',
      from_model: 'sonnet',
      to_model: 'sonnet',
      label: 'effort changed (light)',
      observed_model: 'haiku',
      saved_total: 0,
      saved_short: '0',
    },
  ],
  report: {
    scope: { kind: 'days', days: 30, label: 'last 30 days' },
    compactor: { commands: 12, compacted: 9, raw_bytes: 90000, saved_bytes: 60000, tokens: 15000 },
    tokens_saved_estimate: 73000,
    display: {
      tokens_saved: '73000 tokens',
      tokens_saved_short: '73k',
      scope_label: 'last 30 days',
      routing: '58000 work tokens',
      compactor: '15000 tokens (estimate)',
    },
  },
})

/** Older binary: no report block at all, spawns without saved fields. */
export const PANEL_WITHOUT_REPORT_JSON = JSON.stringify({
  session: '3f9a1c2e',
  switched: 1,
  model_switched: 1,
  effort_changed: 0,
  held: 0,
  spawns: [
    { tool_use: HASH_A, state: 'switched', from_model: 'opus', to_model: 'sonnet', label: 'opus → sonnet (mid)' },
  ],
  tokens_saved_estimate: 0,
})

/** The shape `--no-report` returns: the session view and nothing else. */
export const PANEL_NO_REPORT_JSON = JSON.stringify({
  session: '3f9a1c2e',
  switched: 0,
  model_switched: 0,
  effort_changed: 0,
  held: 0,
  summary: '',
  spawns: [],
})
