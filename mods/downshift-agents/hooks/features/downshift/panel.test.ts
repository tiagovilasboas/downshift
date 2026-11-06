// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { cheaperShare, directLine, routingLines } from '../metrics/normalize'
import { PANEL_JSON } from './panel.fixture'
import { parsePanel, toolUseKey } from './panel'


test('the panel JSON is copied as Downshift states it', () => {
  const s = parsePanel(PANEL_JSON, 5)
  if (s.status !== 'ok') {
    throw new Error(`expected ok, got ${s.status}`)
  }
  expect(s.compactor).toEqual({
    scope: 'last 30 days',
    commands: 494,
    compacted: 46,
    rawBytes: 2837327,
    savedBytes: 57630,
    estimatedTokens: 14407,
    method: 'estimated: saved_bytes / 4',
    families: [
      { family: 'rg', commands: 12, compacted: 11, rawTokens: 4700, avoidedTokens: 1960 },
      { family: 'git-grep', commands: 8, compacted: 6, rawTokens: 2400, avoidedTokens: 800 },
      { family: 'go-test', commands: 5, compacted: 5, rawTokens: 900, avoidedTokens: 120 },
    ],
    direct: {
      scope: 'last 30 days',
      commands: 3,
      savedBytes: 4800,
      estimatedTokens: 1200,
      line: 'direct calls, no session: 3, ~1.2k tokens saved',
    },
  })
  expect(s.tokensSavedScope).toBe('last 30 days, all sessions')
  expect(s.compactor?.direct).toEqual({
    scope: 'last 30 days',
    commands: 3,
    savedBytes: 4800,
    estimatedTokens: 1200,
    line: 'direct calls, no session: 3, ~1.2k tokens saved',
  })
  expect(s.routing.rewrittenSpawns).toBe(1)
  expect(s.routing.rewrittenBillable).toBe(9000)
  expect(s.routing.belowChildren).toBe(1)
  expect(s.routing.belowBillable).toBe(12000)
  expect(s.routing.billable).toBe(30000)
  expect(s.routing.spawns[0]?.reason).toBe('TRIVIAL → small')
  expect(s.routing.spawns[1]?.observed).toBe('claude-sonnet-5-5')
})

test('below-session tokens are kept per harness and read unobserved without data, never summed', () => {
  const s = parsePanel(PANEL_JSON, 5)
  if (s.status !== 'ok') {
    throw new Error(`expected ok, got ${s.status}`)
  }
  expect(s.belowSession.map(r => [r.harness, r.observed, r.belowBillable])).toEqual([
    ['claude-code', true, 12000],
    ['codex', false, 0],
  ])
  expect(routingLines(s.belowSession).map(l => [l.label, l.value])).toEqual([
    ['On a lighter model', '12.0k tokens · 1 of 2 agents'],
    ['On the session model', '18.0k tokens'],
    ['Moved by Downshift', '9.0k tokens · 1 agent'],
    ['codex · no session model', '1.0k of 4.0k tokens lighter than asked (25%) · 1 agent'],
  ])
  expect(cheaperShare(s.belowSession, 'claude-code')).toBe(40)
  expect(cheaperShare(s.belowSession, 'codex')).toBeUndefined()
  // an older Downshift sends no block: one muted line, not a zero
  const raw = JSON.parse(PANEL_JSON) as Record<string, unknown>
  delete raw.below_session_by_harness
  const old = parsePanel(JSON.stringify(raw), 5)
  expect(old.status === 'ok' && old.belowSession.length === 0).toBe(true)
  expect(routingLines([])).toEqual([{ label: 'Not measured yet', value: '', isObserved: false }])
  // a row of the wrong shape is dropped
  raw.below_session_by_harness = [{ harness: 'x', observed: 'yes' }, 5]
  const bad = parsePanel(JSON.stringify(raw), 5)
  expect(bad.status === 'ok' && bad.belowSession.length === 0).toBe(true)
})

test('no exec run means no Compactor block, never zeros', () => {
  const raw = JSON.parse(PANEL_JSON) as Record<string, unknown>
  delete raw.compactor
  const s = parsePanel(JSON.stringify(raw), 5)
  expect(s.status === 'ok' && s.compactor === undefined).toBe(true)
})

test('a Compactor block missing a measured field is dropped', () => {
  const raw = JSON.parse(PANEL_JSON) as { compactor: Record<string, unknown> }
  raw.compactor.raw_bytes = 'lots'
  const s = parsePanel(JSON.stringify(raw), 5)
  expect(s.status === 'ok' && s.compactor === undefined).toBe(true)
})

test('text that is not the panel marks Downshift unavailable', () => {
  expect(parsePanel('downshift: no events', 5).status).toBe('unavailable')
  expect(parsePanel('[]', 5).status).toBe('unavailable')
  expect(parsePanel('{}', 5).status).toBe('unavailable')
})

test('the tool-use key matches Downshift telemetry.ToolUseKey', async () => {
  // sha256("harness-downshift-session-v1:toolu_1"), computed by the Go side.
  expect(await toolUseKey('toolu_1')).toBe(GO_KEY)
})

const GO_KEY = '86aca8a52a293e63e3e5dd26cc8ec12ae04197c80751bdf43806991667a9634e'

test('Compactor families are parsed defensively: missing is empty, malformed dropped, capped at five', () => {
  const families = (value: unknown) => {
    const raw = JSON.parse(PANEL_JSON) as { compactor: Record<string, unknown> }
    if (value === undefined) {
      delete raw.compactor.families
    } else {
      raw.compactor.families = value
    }
    const s = parsePanel(JSON.stringify(raw), 5)
    return s.status === 'ok' ? s.compactor?.families : undefined
  }
  const ok = (family: string) => ({ family, commands: 1, compacted: 1, raw_tokens: 10, avoided_tokens: 5 })
  expect(families(undefined)).toEqual([])
  expect(families('rg')).toEqual([])
  const dirty = [ok('a'), { ...ok('b'), avoided_tokens: -1 }, { ...ok('c'), raw_tokens: 1.5 }, { ...ok('d'), commands: '1' }, 7, { ...ok(''), }, ok('e'), ok('f'), ok('g'), ok('h'), ok('i')]
  expect(families(dirty)?.map(f => f.family)).toEqual(['a', 'e', 'f', 'g', 'h'])
  // a family that cut nothing is not a row: all zero means no block at all
  expect(families([ok('rg'), { ...ok('go-test'), avoided_tokens: 0 }])?.map(f => f.family)).toEqual(['rg'])
  expect(families([{ ...ok('go-test'), avoided_tokens: 0 }, { ...ok('git-log'), avoided_tokens: 0 }])).toEqual([])
})

test('an older panel has no direct block or rewrite figures, and a broken direct block is dropped', () => {
  const raw = JSON.parse(PANEL_JSON) as { compactor: Record<string, unknown>; tokens: Record<string, unknown> } & Record<string, unknown>
  delete raw.tokens.rewritten_spawns
  delete raw.tokens.rewritten_billable
  delete raw.tokens_saved_scope
  raw.compactor.direct = { commands: 'three' }
  const s = parsePanel(JSON.stringify(raw), 5)
  if (s.status !== 'ok') {
    throw new Error(`expected ok, got ${s.status}`)
  }
  expect(s.compactor?.direct).toBeUndefined()
  expect(s.routing.rewrittenSpawns).toBe(0)
  expect(s.tokensSavedScope).toBe('')
})

test('an older binary sends no rewritten fields on a harness row: they read 0 and the row is hidden', () => {
  const raw = JSON.parse(PANEL_JSON) as { below_session_by_harness: Record<string, unknown>[] }
  delete raw.below_session_by_harness[0].rewritten_spawns
  delete raw.below_session_by_harness[0].rewritten_billable
  const s = parsePanel(JSON.stringify(raw), 5)
  if (s.status !== 'ok') {
    throw new Error(`expected ok, got ${s.status}`)
  }
  expect(s.belowSession[0].rewrittenSpawns).toBe(0)
  expect(s.belowSession[0].rewrittenBillable).toBe(0)
  expect(routingLines(s.belowSession).some(l => l.label.includes('Moved by Downshift'))).toBe(false)
})

test('the direct-calls line comes from Downshift figures', () => {
  const s = parsePanel(PANEL_JSON, 5)
  if (s.status !== 'ok' || s.compactor === undefined) {
    throw new Error('expected ok with a compactor')
  }
  expect(directLine(s.compactor)).toEqual({ label: 'Direct calls', value: '3 · ~1.2k* tokens' })
})
