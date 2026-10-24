// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { registerAgents } from '.'

test('agents placeholder registers without hooks', () => {
  let hooks = 0
  registerAgents((() => { hooks += 1 }) as never)
  expect(hooks).toBe(0)
})
