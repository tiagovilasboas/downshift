// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { elapsedSince, formatDuration } from './duration'

test('formatDuration shows seconds, minutes and hours', () => {
  expect(formatDuration(12_000)).toBe('12s')
  expect(formatDuration(0)).toBe('0s')
  expect(formatDuration(185_000)).toBe('3m 05s')
  expect(formatDuration(3_720_000)).toBe('1h 02m')
})

test('formatDuration floors partial seconds and treats bad input as zero', () => {
  expect(formatDuration(1_999)).toBe('1s')
  expect(formatDuration(-5_000)).toBe('0s')
  expect(formatDuration(Number.NaN)).toBe('0s')
  expect(formatDuration(Number.POSITIVE_INFINITY)).toBe('0s')
})

test('elapsedSince measures from a timestamp and never goes negative', () => {
  expect(elapsedSince(1_000, 13_000)).toBe('12s')
  expect(elapsedSince(5_000, 1_000)).toBe('0s')
})
