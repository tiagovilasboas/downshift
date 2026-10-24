// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

test('/downshift-progress is registered and answers one line', async ($, on) => {
  const registered: string[] = []
  on('command.register', (_$: any, e: any) => { registered.push(e.name); return { value: { command: e.name } } })
  on('session.start', (_$: any, e: any) => ({ cwd: e.cwd }))

  await $.session.start({ cwd: '/', surface: 'terminal', isInteractive: true })
  expect(registered).toEqual(['downshift-progress'])

  const r = await $.command.run({ command: 'downshift-progress', args: '' } as Parameters<typeof $.command.run>[0])
  expect(JSON.stringify(r)).toContain('downshift-progress 0.1.0 loaded')
})
