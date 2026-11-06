// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Reads the JSON of `downshift session --panel` (cmd/downshift/sessionpanel.go)
// into the mod's snapshot. Downshift owns every figure: this file copies
// fields, it computes none. A field of the wrong type drops its block rather
// than show a made-up zero.

import type { BelowSessionRow, CompactorDirect, CompactorFamily, CompactorFigures, DownshiftSnapshot, EfficiencyRow, RoutingFigures, RoutingSpawn } from '../../../types'

type Json = Record<string, unknown>

const isObject = (v: unknown): v is Json => typeof v === 'object' && v !== null && !Array.isArray(v)
const num = (o: Json, k: string): number | undefined => (typeof o[k] === 'number' ? (o[k] as number) : undefined)
const str = (o: Json, k: string): string | undefined => (typeof o[k] === 'string' ? (o[k] as string) : undefined)

export function parsePanel(stdout: string, now: number): DownshiftSnapshot {
  let raw: unknown
  try {
    raw = JSON.parse(stdout)
  } catch {
    return { status: 'unavailable', reason: 'Downshift answered something that is not JSON', at: now }
  }
  if (!isObject(raw)) {
    return { status: 'unavailable', reason: 'Downshift answered an unexpected shape', at: now }
  }
  const routing = routingOf(raw)
  if (routing === undefined) {
    return { status: 'unavailable', reason: 'Downshift panel is missing its session counts', at: now }
  }
  return {
    status: 'ok',
    at: now,
    routing,
    compactor: compactorOf(raw.compactor),
    tokensSavedScope: str(raw, 'tokens_saved_scope') ?? '',
    belowSession: belowSessionOf(raw.below_session_by_harness),
    efficiency: efficiencyOf(raw.efficiency),
  }
}

function routingOf(o: Json): RoutingFigures | undefined {
  const tokens = isObject(o.tokens) ? o.tokens : {}
  const switched = num(o, 'switched')
  const held = num(o, 'held')
  if (switched === undefined || held === undefined) {
    return undefined
  }
  return {
    switched,
    modelSwitched: num(o, 'model_switched') ?? 0,
    effortChanged: num(o, 'effort_changed') ?? 0,
    held,
    observed: num(o, 'observed') ?? 0,
    children: num(tokens, 'spawns') ?? 0,
    belowChildren: num(tokens, 'below_spawns') ?? 0,
    billable: num(tokens, 'billable') ?? 0,
    belowBillable: num(tokens, 'below_billable') ?? 0,
    rewrittenSpawns: num(tokens, 'rewritten_spawns') ?? 0,
    rewrittenBillable: num(tokens, 'rewritten_billable') ?? 0,
    line: str(o, 'tokens_line') ?? '',
    spawns: spawnsOf(o.spawns, o.routing_why),
  }
}

function spawnsOf(spawns: unknown, why: unknown): RoutingSpawn[] {
  const reasons = new Map<string, string>()
  for (const w of Array.isArray(why) ? why : []) {
    if (isObject(w) && str(w, 'tool_use') !== undefined) {
      reasons.set(str(w, 'tool_use') ?? '', str(w, 'cell') ?? '')
    }
  }
  const out: RoutingSpawn[] = []
  for (const s of Array.isArray(spawns) ? spawns : []) {
    if (!isObject(s)) {
      continue
    }
    const key = str(s, 'tool_use') ?? ''
    out.push({
      toolUseKey: key,
      state: str(s, 'state') ?? 'unknown',
      requested: str(s, 'from_model') ?? 'unknown',
      final: str(s, 'to_model') ?? 'unknown',
      observed: str(s, 'observed_model'),
      label: str(s, 'label') ?? '',
      reason: reasons.get(key) || undefined,
    })
  }
  return out.slice(-100)
}

/** One row per harness, as Downshift counted it; a row of the wrong shape is dropped, not zeroed. */
function belowSessionOf(rows: unknown): BelowSessionRow[] {
  const out: BelowSessionRow[] = []
  for (const r of Array.isArray(rows) ? rows : []) {
    const harness = isObject(r) ? str(r, 'harness') : undefined
    if (!isObject(r) || harness === undefined || typeof r.observed !== 'boolean') {
      continue
    }
    out.push({
      harness,
      observed: r.observed,
      spawns: num(r, 'spawns') ?? 0,
      belowSpawns: num(r, 'below_spawns') ?? 0,
      billable: num(r, 'billable') ?? 0,
      belowBillable: num(r, 'below_billable') ?? 0,
      rewrittenSpawns: num(r, 'rewritten_spawns') ?? 0,
      rewrittenBillable: num(r, 'rewritten_billable') ?? 0,
      fallbackSpawns: num(r, 'fallback_spawns') ?? 0,
      fallbackBillable: num(r, 'fallback_billable') ?? 0,
      fallbackBelowBillable: num(r, 'fallback_below_billable') ?? 0,
      unobservedSpawns: num(r, 'unobserved_spawns') ?? 0,
      cacheRead: num(r, 'cache_read') ?? 0,
    })
  }
  return out
}

/**
 * The `efficiency` block (accounting.Efficiency), one row per harness. A row
 * with no harness, no premium flag or no spawn count is dropped, not zeroed.
 */
function efficiencyOf(block: unknown): EfficiencyRow[] {
  const rows = isObject(block) && Array.isArray(block.harnesses) ? block.harnesses : []
  const out: EfficiencyRow[] = []
  for (const r of rows) {
    if (!isObject(r) || str(r, 'harness') === undefined || typeof r.premium_observed !== 'boolean' || num(r, 'spawns') === undefined) {
      continue
    }
    const tiers = isObject(r.spawns_by_tier) ? r.spawns_by_tier : {}
    const count = (k: string) => (isObject(tiers[k]) ? num(tiers[k] as Json, 'count') ?? 0 : 0)
    out.push({
      harness: str(r, 'harness') ?? '',
      spawns: num(r, 'spawns') ?? 0,
      byTier: { small: count('small'), mid: count('mid'), frontier: count('frontier'), unknown: count('unknown') },
      ended: num(r, 'ended') ?? 0,
      completed: num(r, 'completed_tasks') ?? 0,
      reworked: num(r, 'reworked') ?? 0,
      reworkRatio: num(r, 'rework_ratio') ?? 0,
      premiumObserved: r.premium_observed,
      premiumPct: num(r, 'premium_preserved_pct') ?? 0,
      tierKnown: num(r, 'completed_tier_known') ?? 0,
      nonFrontier: num(r, 'completed_non_frontier') ?? 0,
      tierUnknown: num(r, 'completed_tier_unknown') ?? 0,
    })
  }
  return out
}

function compactorOf(c: unknown): CompactorFigures | undefined {
  if (!isObject(c)) {
    return undefined
  }
  const rawBytes = num(c, 'raw_bytes')
  const savedBytes = num(c, 'saved_bytes')
  const estimatedTokens = num(c, 'estimated_tokens_saved')
  const method = str(c, 'token_estimate')
  if (rawBytes === undefined || savedBytes === undefined || estimatedTokens === undefined || method === undefined) {
    return undefined
  }
  return {
    scope: str(c, 'scope') ?? '',
    commands: num(c, 'commands') ?? 0,
    compacted: num(c, 'compacted') ?? 0,
    rawBytes,
    savedBytes,
    estimatedTokens,
    method,
    families: familiesOf(c.families),
    direct: directOf(c.direct),
  }
}

/** Older binaries send none; a block missing a figure is dropped, not zeroed. */
function directOf(d: unknown): CompactorDirect | undefined {
  if (!isObject(d)) {
    return undefined
  }
  const commands = num(d, 'commands')
  const savedBytes = num(d, 'saved_bytes')
  const estimatedTokens = num(d, 'estimated_tokens_saved')
  const line = str(d, 'line')
  if (commands === undefined || savedBytes === undefined || estimatedTokens === undefined || line === undefined) {
    return undefined
  }
  return { scope: str(d, 'scope') ?? '', commands, savedBytes, estimatedTokens, line }
}

const MAX_FAMILIES = 5

const count = (v: unknown): number | undefined => (typeof v === 'number' && Number.isInteger(v) && v >= 0 ? v : undefined)

/**
 * Older binaries send none: that is an empty list. Malformed entries are
 * dropped, and so are families that cut nothing: a row at 0% is noise in the
 * card. The order given (most avoided first) is kept.
 */
function familiesOf(list: unknown): CompactorFamily[] {
  if (!Array.isArray(list)) {
    return []
  }
  const out: CompactorFamily[] = []
  for (const f of list) {
    if (!isObject(f)) {
      continue
    }
    const family = str(f, 'family')
    const commands = count(f.commands)
    const compacted = count(f.compacted)
    const rawTokens = count(f.raw_tokens)
    const avoidedTokens = count(f.avoided_tokens)
    if (family === undefined || family === '' || commands === undefined || compacted === undefined || rawTokens === undefined || avoidedTokens === undefined) {
      continue
    }
    if (avoidedTokens === 0) {
      continue
    }
    out.push({ family, commands, compacted, rawTokens, avoidedTokens })
  }
  return out.slice(0, MAX_FAMILIES)
}

/**
 * Downshift's key for a harness tool call id (telemetry.ToolUseKey): hex
 * sha256 of a fixed prefix and the id, documented for readers that know the id.
 */
export async function toolUseKey(toolUseId: string): Promise<string> {
  const bytes = new TextEncoder().encode(`harness-downshift-session-v1:${toolUseId}`)
  const sum = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
  return Array.from(sum, b => b.toString(16).padStart(2, '0')).join('')
}
