// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { EfficiencyRow } from '../../../types'
import { parsePanel } from '../downshift/panel'
import { PANEL_JSON } from '../downshift/panel.fixture'
import { efficiencyLines, premiumText, reworkText } from './efficiency'

const row = (extra: Partial<EfficiencyRow> = {}): EfficiencyRow => ({
  harness: 'claude-code',
  spawns: 4,
  byTier: { small: 2, mid: 1, frontier: 1, unknown: 0 },
  ended: 4,
  completed: 4,
  reworked: 0,
  reworkRatio: 0,
  premiumObserved: true,
  premiumPct: 75,
  tierKnown: 4,
  nonFrontier: 3,
  tierUnknown: 0,
  ...extra,
})

test('the efficiency block is kept per harness, in the words of downshift report', () => {
  const s = parsePanel(PANEL_JSON, 1)
  if (s.status !== 'ok') {
    throw new Error('fixture must parse')
  }
  expect(efficiencyLines(s.efficiency)).toEqual([
    {
      harness: 'claude-code',
      premium: '75% (3 of 4 completed tasks ran below frontier)',
      isPremiumObserved: true,
      tiers: 'small 2 · mid 1 · frontier 1 · unknown 1',
      completed: '4 of 4 ended',
      rework: '25% (1 of 4 ended)',
    },
    {
      harness: 'codex',
      premium: 'unobserved',
      isPremiumObserved: false,
      tiers: 'small 0 · mid 0 · frontier 0 · unknown 1',
      completed: '0 of 0 ended',
      rework: 'unobserved (no spawn ended)',
    },
  ])
})

test('premium preserved reads unobserved without a tier-known task, never 0%', () => {
  expect(premiumText(row({ premiumObserved: false, premiumPct: 0, tierKnown: 0, nonFrontier: 0 }))).toBe('unobserved')
  expect(premiumText(row({ premiumObserved: false, tierKnown: 0, nonFrontier: 0, tierUnknown: 2 }))).toBe('unobserved (2 completed with unknown tier)')
  expect(premiumText(row({ tierUnknown: 2 }))).toBe('75% (3 of 4 completed tasks ran below frontier); 2 with unknown tier left out')
  expect(premiumText(row({ premiumPct: 66.666, nonFrontier: 2, tierKnown: 3 }))).toBe('67% (2 of 3 completed tasks ran below frontier)')
  expect(reworkText(row({ reworked: 1, reworkRatio: 0.25 }))).toBe('25% (1 of 4 ended)')
})

test('no money and no tokens in any efficiency line', () => {
  const s = parsePanel(PANEL_JSON, 1)
  const text = JSON.stringify(s.status === 'ok' ? efficiencyLines(s.efficiency) : [])
  expect(/\$|USD|cost|token/i.test(text)).toBe(false)
})

test('a panel without the efficiency block, or with a malformed row, has no rows rather than zeros', () => {
  const old = parsePanel(JSON.stringify({ ...JSON.parse(PANEL_JSON), efficiency: undefined }), 1)
  expect(old.status === 'ok' && old.efficiency.length === 0).toBe(true)
  const bad = parsePanel(JSON.stringify({ ...JSON.parse(PANEL_JSON), efficiency: { harnesses: [{ harness: 'x', spawns: 1 }, 'nope'] } }), 1)
  expect(bad.status === 'ok' && bad.efficiency.length === 0).toBe(true)
})
