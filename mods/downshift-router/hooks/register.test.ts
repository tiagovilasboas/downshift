// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
//
// Tests for the downshift-router mod against the mock engine
// (`claude plugin test mods/downshift-router`). The binary is stubbed through
// `process.run`, so these tests prove the adapter contract only: what is sent
// on stdin, what is read back, and that every failure leaves the spawn as it was.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import { describe, expect, test } from 'claude-code/testing'
import type { AgentSpawnInput, On, ProcessRunInit } from 'claude-code'

import { buildPayload, isRoutable, modelFromHookOutput } from './register'

const spawn = (over: Partial<AgentSpawnInput> = {}): AgentSpawnInput => ({
  tool_use_id: 'toolu_01',
  prompt: 'List the files under src/ and report their names. Do not change anything.',
  description: 'List files',
  subagentType: 'general-purpose',
  provider: { plugin: 'engine', tier: 'core' },
  parentModel: 'claude-opus-4-1',
  permissionMode: 'default',
  background: false,
  fork: false,
  ...over,
})

type Run = { argv: readonly string[]; init?: ProcessRunInit }
type Answer = { exitCode?: number; stdout?: string; stderr?: string } | { deny: string }

const rewrite = (model: string) => JSON.stringify({
  hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'allow', updatedInput: { prompt: 'x', description: 'y', model } },
})
const allowUnchanged = JSON.stringify({ hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'allow' } })

// The world beneath the mod: the environment, the session, a scripted binary
// and the engine's own answer to agent.spawn, which records the model it got.
function world(on: On, answer: Answer, opts: { env?: Record<string, string> } = {}) {
  const runs: Run[] = []
  const logs: string[] = []
  const bottom: AgentSpawnInput[] = []
  on('env.get', ($, e) => ({ value: opts.env?.[e.name] }))
  on('session.id', () => ({ value: 'S1' }))
  on('session.cwd', () => ({ value: '/work' }))
  on('ui.log', ($, e) => { logs.push(e.text); return { value: undefined } })
  on('process.run', ($, e) => {
    runs.push({ argv: e.argv, init: e.init })
    if ('deny' in answer) return { deny: answer.deny }
    return { value: { exitCode: answer.exitCode ?? 0, stdout: answer.stdout ?? '', stderr: answer.stderr ?? '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  on('agent.spawn', ($, e) => { bottom.push(e); return { model: e.model ?? e.parentModel, agentId: 'A1' } })
  return { runs, logs, bottom }
}

describe('downshift-router: agent.spawn adapter', () => {
  test('disabled by default: the spawn passes through and the binary never runs', async ($, on) => {
    const w = world(on, { stdout: rewrite('haiku') })
    const result = await $.agent.spawn(spawn())
    expect(result).toEqual({ model: 'claude-opus-4-1', agentId: 'A1' })
    expect(w.runs.length).toBe(0)
    expect(w.bottom[0]?.model).toBe(undefined)
    expect(w.logs).toEqual([])
  })

  test('enabled by userConfig: a rewrite from the binary becomes the spawn model', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { stdout: rewrite('haiku'), stderr: 'downshift: correlation_id=abc TRIVIAL task → downshift to haiku\n' })
    const result = await $.agent.spawn(spawn())
    expect(result.model).toBe('haiku')
    expect(w.bottom[0]?.model).toBe('haiku')
    expect(w.bottom[0]?.prompt).toBe(spawn().prompt)
    expect(w.runs.length).toBe(1)
    expect(w.runs[0]?.argv).toEqual(['downshift', 'claude-code'])
    expect(w.runs[0]?.init?.timeoutMs).toBe(8000)
    const payload = JSON.parse(w.runs[0]?.init?.stdin ?? '{}')
    expect(payload.hook_event_name).toBe('PreToolUse')
    expect(payload.tool_name).toBe('Agent')
    expect(payload.tool_use_id).toBe('toolu_01')
    expect(payload.session_id).toBe('S1')
    expect(payload.cwd).toBe('/work')
    expect(payload.permission_mode).toBe('default')
    expect(payload.parent_model).toBe('claude-opus-4-1')
    expect(payload.entry).toBe('mod')
    expect(payload.tool_input).toEqual({ prompt: spawn().prompt, description: 'List files', subagent_type: 'general-purpose', run_in_background: false })
    expect(w.logs.length).toBe(1)
    expect(w.logs[0]).toContain('→ haiku')
  })

  test('enabled by DOWNSHIFT_MOD_ROUTER=1 when the option is off', async ($, on) => {
    const w = world(on, { stdout: rewrite('haiku') }, { env: { DOWNSHIFT_MOD_ROUTER: '1' } })
    const result = await $.agent.spawn(spawn({ model: 'opus' }))
    expect(result.model).toBe('haiku')
    expect(w.runs.length).toBe(1)
    expect(JSON.parse(w.runs[0]?.init?.stdin ?? '{}').tool_input.model).toBe('opus')
  })

  test('the configured binary path is what runs', { options: { enabled: true, binary: '/opt/bin/downshift' } }, async ($, on) => {
    const w = world(on, { stdout: allowUnchanged })
    await $.agent.spawn(spawn())
    expect(w.runs[0]?.argv).toEqual(['/opt/bin/downshift', 'claude-code'])
  })

  test('binary error: the spawn is unchanged and one debug line is logged', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { deny: 'spawn ENOENT' })
    const result = await $.agent.spawn(spawn())
    expect(result).toEqual({ model: 'claude-opus-4-1', agentId: 'A1' })
    expect(w.bottom[0]?.model).toBe(undefined)
    expect(w.logs.length).toBe(1)
    expect(w.logs[0]).toContain('left unchanged')
    expect(w.logs[0]).toContain('spawn ENOENT')
  })

  test('non-zero exit: unchanged, never a deny', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { exitCode: 2, stdout: '', stderr: 'blocked' })
    const result = await $.agent.spawn(spawn({ model: 'sonnet' }))
    expect(result.model).toBe('sonnet')
    expect(w.bottom[0]?.model).toBe('sonnet')
    expect(w.logs[0]).toContain('exit 2')
  })

  test('non-JSON output: unchanged', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { stdout: 'not json at all' })
    const result = await $.agent.spawn(spawn())
    expect(result).toEqual({ model: 'claude-opus-4-1', agentId: 'A1' })
    expect(w.bottom[0]?.model).toBe(undefined)
    expect(w.logs[0]).toContain('no model in hook output')
  })

  test('allow without updatedInput: unchanged and quiet', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { stdout: allowUnchanged, stderr: 'downshift: correlation_id=abc OK\n' })
    const result = await $.agent.spawn(spawn())
    expect(result.model).toBe('claude-opus-4-1')
    expect(w.bottom[0]?.model).toBe(undefined)
  })

  test('the same model written back is not a rewrite', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { stdout: rewrite('sonnet') })
    await $.agent.spawn(spawn({ model: 'sonnet' }))
    expect(w.bottom[0]?.model).toBe('sonnet')
    expect(w.logs).toEqual([])
  })

  test('a fork, a resume and a workflow agent are passed through without running the binary', { options: { enabled: true } }, async ($, on) => {
    const w = world(on, { stdout: rewrite('haiku') })
    await $.agent.spawn(spawn({ fork: true }))
    await $.agent.spawn(spawn({ resume: { agentId: 'A0' } }))
    await $.agent.spawn(spawn({ workflow: { runId: 'wf_1', agentIndex: 1 } }))
    expect(w.runs.length).toBe(0)
    expect(w.bottom.map(e => e.model)).toEqual([undefined, undefined, undefined])
  })

  test('DOWNSHIFT_MOD_ROUTER_LOG appends one JSON line per spawn', { options: { enabled: true } }, async ($, on) => {
    const files = new Map<string, string>()
    on('fs.read', ($, e) => {
      const text = files.get(e.path)
      if (text === undefined) throw new Error('ENOENT')
      return { value: text }
    })
    on('fs.write', ($, e) => { files.set(e.path, e.text); return { value: undefined } })
    world(on, { stdout: rewrite('haiku') }, { env: { DOWNSHIFT_MOD_ROUTER_LOG: '/tmp/router.jsonl' } })
    await $.agent.spawn(spawn())
    await $.agent.spawn(spawn({ tool_use_id: 'toolu_02' }))
    const lines = (files.get('/tmp/router.jsonl') ?? '').trim().split('\n').map(l => JSON.parse(l))
    expect(lines.length).toBe(2)
    expect(lines[0].tool_use_id).toBe('toolu_01')
    expect(lines[0].written_model).toBe('haiku')
    expect(lines[1].tool_use_id).toBe('toolu_02')
  })
})

describe('downshift-router: pure helpers', () => {
  test('buildPayload keeps only what the event carries', async () => {
    const p = buildPayload(spawn({ subagentType: '', permissionMode: undefined }), undefined, undefined)
    expect(p).toEqual({
      hook_event_name: 'PreToolUse', tool_name: 'Agent', tool_use_id: 'toolu_01', parent_model: 'claude-opus-4-1',
      tool_input: { prompt: spawn().prompt, description: 'List files', run_in_background: false },
    })
  })

  test('modelFromHookOutput reads the rewrite and rejects everything else', async () => {
    expect(modelFromHookOutput(rewrite('haiku'))).toBe('haiku')
    expect(modelFromHookOutput(allowUnchanged)).toBe(null)
    expect(modelFromHookOutput('')).toBe(null)
    expect(modelFromHookOutput('[]')).toBe(null)
    expect(modelFromHookOutput(JSON.stringify({ hookSpecificOutput: { updatedInput: { model: 7 } } }))).toBe(null)
  })

  test('isRoutable', async () => {
    expect(isRoutable(spawn())).toBe(true)
    expect(isRoutable(spawn({ fork: true }))).toBe(false)
    expect(isRoutable(spawn({ resume: { agentId: 'A0' } }))).toBe(false)
    expect(isRoutable(spawn({ workflow: { runId: 'wf', agentIndex: 1 } }))).toBe(false)
  })
})
