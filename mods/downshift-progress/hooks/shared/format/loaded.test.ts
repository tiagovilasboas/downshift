// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { loadedLine } from './loaded'

test('loadedLine joins name and version', () => {
  expect(loadedLine('downshift-progress', '0.1.0')).toBe('downshift-progress 0.1.0 loaded')
})
