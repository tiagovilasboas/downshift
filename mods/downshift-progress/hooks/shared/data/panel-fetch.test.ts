// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The test engine's `$` carries no `$.process`, so the host is stubbed here: the
// stub records each argv and answers or rejects as the case needs.

import { expect, test } from 'claude-code/testing'

import { PANEL_TIMEOUT_MS, classifyFailure, fetchPanel } from '.'
import type { HostPort, RunResult } from '.'
import { PANEL_NO_REPORT_JSON, TODAY_PANEL_JSON } from './panel-fixtures'

type Call = { argv: string[]; timeoutMs?: number }

type Stub = { host: HostPort; calls: Call[] }

/** A host whose `$.process.run` answers `reply`: a result, or a thrown error. */
const stubHost = (reply: RunResult | Error, env: Record<string, string> = {}): Stub => {
  const calls: Call[] = []
  const host: HostPort = {
    process: {
      run: async (argv, init) => {
        calls.push({ argv: [...argv], timeoutMs: init?.timeoutMs })
        if (reply instanceof Error) throw reply
        return reply
      },
    },
    env: { get: async (name) => env[name] },
  }
  return { host, calls }
}

const exit0 = (stdout: string): RunResult => ({ exitCode: 0, stdout, stderr: '' })

test('runs session --panel with the session id and a 2 s budget, then parses', async () => {
  const { host, calls } = stubHost(exit0(TODAY_PANEL_JSON))
  const r = await fetchPanel(host, '3f9a1c2e-raw', { withReport: true, binary: 'downshift' })
  expect(calls[0]?.argv).toEqual(['downshift', 'session', '--panel', '--session=3f9a1c2e-raw'])
  expect(calls[0]?.timeoutMs).toBe(2000)
  expect(PANEL_TIMEOUT_MS).toBe(2000)
  expect(r.ok).toBe(true)
  if (r.ok) expect(r.panel.report?.tokensSavedEstimate).toBe(63000)
})

test('withReport false adds --no-report', async () => {
  const { host, calls } = stubHost(exit0(PANEL_NO_REPORT_JSON))
  const r = await fetchPanel(host, 'abc', { withReport: false, binary: 'downshift' })
  expect(calls[0]?.argv).toEqual(['downshift', 'session', '--panel', '--session=abc', '--no-report'])
  expect(r.ok && r.panel.report).toBeFalsy()
})

test('defaults to downshift on PATH; DOWNSHIFT_BIN overrides it; an explicit option wins', async () => {
  const plain = stubHost(exit0(PANEL_NO_REPORT_JSON))
  await fetchPanel(plain.host, 'abc', { withReport: false })
  expect(plain.calls[0]?.argv[0]).toBe('downshift')

  const env = stubHost(exit0(PANEL_NO_REPORT_JSON), { DOWNSHIFT_BIN: '/opt/ds/downshift' })
  await fetchPanel(env.host, 'abc', { withReport: false })
  expect(env.calls[0]?.argv[0]).toBe('/opt/ds/downshift')

  await fetchPanel(env.host, 'abc', { withReport: false, binary: '/custom/downshift' })
  expect(env.calls[1]?.argv[0]).toBe('/custom/downshift')
})

test('no session id answers no-session without running anything', async () => {
  const { host, calls } = stubHost(exit0(TODAY_PANEL_JSON))
  expect(await fetchPanel(host, null, { withReport: true })).toEqual({ ok: false, reason: 'no-session' })
  expect(await fetchPanel(host, '', { withReport: true })).toEqual({ ok: false, reason: 'no-session' })
  expect(calls.length).toBe(0)
})

test('a binary that cannot start is not-installed', async () => {
  const { host } = stubHost(Object.assign(new Error('spawn downshift ENOENT'), { code: 'ENOENT' }))
  expect(await fetchPanel(host, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'not-installed' })
})

test('a call still running at the budget is timeout', async () => {
  const { host } = stubHost(new Error('downshift timed out after 2000 ms'))
  expect(await fetchPanel(host, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'timeout' })
})

test('another rejection, or a non-zero exit, is unavailable-surface', async () => {
  const rejected = stubHost(new Error('host commands are not available on this surface'))
  expect(await fetchPanel(rejected.host, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'unavailable-surface' })
  const usage = stubHost({ exitCode: 2, stdout: '', stderr: 'usage: downshift session' })
  expect(await fetchPanel(usage.host, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'unavailable-surface' })
})

test('empty or garbage stdout is bad-json', async () => {
  expect(await fetchPanel(stubHost(exit0('')).host, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'bad-json' })
  expect(await fetchPanel(stubHost(exit0('<html>no</html>')).host, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'bad-json' })
})

test('a surface without $.process answers unavailable-surface and never throws', async () => {
  expect(await fetchPanel({}, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'unavailable-surface' })
  const broken: HostPort = { process: { run: () => Promise.reject(new Error('boom')) } }
  expect(await fetchPanel(broken, 'abc', { withReport: true })).toEqual({ ok: false, reason: 'unavailable-surface' })
  const badEnv: HostPort = {
    process: { run: async () => exit0(TODAY_PANEL_JSON) },
    env: { get: () => Promise.reject(new Error('no env here')) },
  }
  expect((await fetchPanel(badEnv, 'abc', { withReport: true })).ok).toBe(true)
})

test('classifyFailure reads the code and the message of a rejection', () => {
  expect(classifyFailure(Object.assign(new Error('x'), { code: 'ENOENT' }))).toBe('not-installed')
  expect(classifyFailure(new Error('command cannot start'))).toBe('not-installed')
  expect(classifyFailure(new Error('still running at 2000 ms'))).toBe('timeout')
  expect(classifyFailure('plain string')).toBe('unavailable-surface')
})
