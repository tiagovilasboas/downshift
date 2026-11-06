// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The meter and the status looks: pure presentation, tested without a surface.

import { expect, test } from 'claude-code/testing'

import { ACCENT, DONE, FAIL, MUTED, WARN, meter } from './format'
import { lookOf } from './status'

const cells = (m: { fill: string; track: string }) => [...m.fill].length + [...m.track].length

test('the meter keeps its width at every share, with a half cell between two', () => {
  for (const p of [undefined, 0, 1, 33, 57, 99, 100, 140, -5]) {
    expect(cells(meter(8, p))).toBe(8)
  }
  expect(meter(8, undefined).fill).toBe('')
  expect(meter(8, 0).fill).toBe('')
  expect(meter(8, 100).track).toBe('')
  expect(meter(8, 57).fill).toBe('━━━━╸')
  expect(meter(0, 50)).toEqual({ fill: '╸', track: '' })
})

test('each status has its own mark, and one color per meaning', () => {
  const statuses = ['pending', 'running', 'completed', 'failed', 'blocked', 'cancelled'] as const
  expect(new Set(statuses.map(s => lookOf(s).mark)).size).toBe(statuses.length)
  expect(lookOf('running').color).toBe(ACCENT)
  expect(lookOf('completed').color).toBe(DONE)
  expect(lookOf('failed').color).toBe(FAIL)
  expect(lookOf('blocked').color).toBe(WARN)
  expect(lookOf('pending').color).toBe(MUTED)
})
