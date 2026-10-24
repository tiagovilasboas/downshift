// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import {
  PANEL_NO_REPORT_JSON,
  PANEL_WITH_NEW_FIELDS_JSON,
  PANEL_WITHOUT_REPORT_JSON,
  TODAY_PANEL_JSON,
} from './panel-fixtures'
import { parsePanel } from './panel-parse'

test('today JSON: session view and report parse, new fields are null', () => {
  const p = parsePanel(TODAY_PANEL_JSON)
  expect(p).not.toBeNull()
  expect(p?.session).toBe('3f9a1c2e')
  expect(p?.switched).toBe(1)
  expect(p?.held).toBe(1)
  expect(p?.summary).toBeNull()
  expect(p?.spawns.length).toBe(2)
  expect(p?.spawns[0]?.state).toBe('switched')
  expect(p?.spawns[0]?.label).toBe('opus → sonnet (mid)')
  expect(p?.spawns[0]?.fromModel).toBe('opus')
  expect(p?.spawns[0]?.toModel).toBe('sonnet')
  expect(p?.spawns[0]?.savedTotal).toBeNull()
  expect(p?.spawns[0]?.savedShort).toBeNull()
  expect(p?.report?.tokensSavedEstimate).toBe(63000)
  expect(p?.report?.scope).toEqual({ kind: 'days', days: 30, label: null })
  expect(p?.report?.display.tokensSavedShort).toBeNull()
  expect(p?.report?.display.scopeLabel).toBeNull()
  expect(p?.report?.display.tokensSaved).toBe('63000 tokens')
  expect(p?.report?.compactor).toEqual({ commands: 12, compacted: 9, savedBytes: 60000, tokens: 15000 })
})

test('JSON with the new fields: saved figures, summary, display short and scope label', () => {
  const p = parsePanel(PANEL_WITH_NEW_FIELDS_JSON)
  expect(p?.summary).toBe('Downshift · 1 model · 1 effort')
  expect(p?.spawns[0]?.savedTotal).toBe(58000)
  expect(p?.spawns[0]?.savedShort).toBe('58k')
  expect(p?.spawns[1]?.observedModel).toBe('haiku')
  expect(p?.report?.display.tokensSavedShort).toBe('73k')
  expect(p?.report?.display.scopeLabel).toBe('last 30 days')
  expect(p?.report?.scope.label).toBe('last 30 days')
})

test('older JSON without a report: report is null, spawns still parse', () => {
  const p = parsePanel(PANEL_WITHOUT_REPORT_JSON)
  expect(p?.report).toBeNull()
  expect(p?.spawns.length).toBe(1)
  expect(p?.spawns[0]?.toolUse).toBe('a'.repeat(64))
})

test('--no-report shape: empty summary reads as null, empty spawns', () => {
  const p = parsePanel(PANEL_NO_REPORT_JSON)
  expect(p?.summary).toBeNull()
  expect(p?.spawns).toEqual([])
  expect(p?.report).toBeNull()
})

test('garbage answers null and never throws', () => {
  for (const bad of ['', '   ', 'not json', '{', '[]', 'null', '42', '"text"', '{}', '{"x":1}']) {
    expect(parsePanel(bad)).toBeNull()
  }
})

test('mistyped fields become null; non-object spawns are dropped', () => {
  const p = parsePanel(
    JSON.stringify({
      session: 7,
      switched: '2',
      held: Number.NaN,
      spawns: [null, 'x', { state: 'held', saved_total: '58k', saved_short: '' }],
      report: 'nope',
    }),
  )
  expect(p).not.toBeNull()
  expect(p?.session).toBeNull()
  expect(p?.switched).toBeNull()
  expect(p?.held).toBeNull()
  expect(p?.spawns.length).toBe(1)
  expect(p?.spawns[0]?.savedTotal).toBeNull()
  expect(p?.spawns[0]?.savedShort).toBeNull()
  expect(p?.report).toBeNull()
})
