// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
//
// downshift-router: an experimental Claude Code mod that routes a subagent's
// model from the `agent.spawn` event instead of (or beside) the settings
// PreToolUse hook. It is an adapter and nothing more: it turns the event into
// the PreToolUse-shaped payload the `downshift claude-code` hook already reads
// on stdin, runs the binary, and applies `hookSpecificOutput.updatedInput.model`.
// Classification, policy, session rules and telemetry stay in the Go core
// (docs/contrib/layering.md). Off unless enabled; fails open; never denies.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { AgentSpawnInput, EngineInterface, PluginOptions, Register } from 'claude-code'

/** A safety net above the binary's own budgets (2 s stdin read, 1.5 s classification deadline). */
const RUN_TIMEOUT_MS = 8000
const LOG_PREFIX = 'downshift-router'

/** The PreToolUse-shaped payload `downshift claude-code` reads on stdin. */
export type HookPayload = {
  hook_event_name: 'PreToolUse'
  tool_name: 'Agent'
  tool_use_id: string
  session_id?: string
  cwd?: string
  permission_mode?: string
  /**
   * The parent's effective model, a fact the settings hook has to recover from
   * the transcript tail. The Claude Code adapter reads it under its inheriting
   * rule (before the transcript), so a mod spawn gets a known baseline without
   * waiting on the transcript file.
   */
  parent_model?: string
  /**
   * Which spawn-time path built this payload. The shared runner records it on
   * the decision event so stats and doctor can tell hook and mod apart.
   */
  entry?: 'mod'
  tool_input: {
    prompt: string
    description: string
    subagent_type?: string
    model?: string
    name?: string
    run_in_background?: boolean
    cwd?: string
  }
}

/** Builds the payload from the fields the event really carries; nothing is invented. */
export function buildPayload(e: AgentSpawnInput, sessionId: string | undefined, cwd: string | undefined): HookPayload {
  const payload: HookPayload = {
    hook_event_name: 'PreToolUse',
    tool_name: 'Agent',
    tool_use_id: e.tool_use_id,
    tool_input: { prompt: e.prompt, description: e.description },
  }
  if (sessionId) payload.session_id = sessionId
  if (cwd) payload.cwd = cwd
  if (e.permissionMode) payload.permission_mode = e.permissionMode
  if (e.parentModel) payload.parent_model = e.parentModel
  payload.entry = 'mod'
  if (e.subagentType) payload.tool_input.subagent_type = e.subagentType
  if (e.model) payload.tool_input.model = e.model
  if (e.name) payload.tool_input.name = e.name
  if (typeof e.background === 'boolean') payload.tool_input.run_in_background = e.background
  if (e.cwd) payload.tool_input.cwd = e.cwd
  return payload
}

/** Reads `hookSpecificOutput.updatedInput.model` from the hook's stdout; null when absent or not JSON. */
export function modelFromHookOutput(stdout: string): string | null {
  let out: unknown
  try {
    out = JSON.parse(stdout)
  } catch {
    return null
  }
  if (!out || typeof out !== 'object') return null
  const hso = (out as { hookSpecificOutput?: unknown }).hookSpecificOutput
  if (!hso || typeof hso !== 'object') return null
  const updated = (hso as { updatedInput?: unknown }).updatedInput
  if (!updated || typeof updated !== 'object') return null
  const model = (updated as { model?: unknown }).model
  return typeof model === 'string' && model !== '' ? model : null
}

/** Spawns the event cannot or should not route: a fork inherits and ignores `model`; a resume only accepts a refusal; a workflow agent's prompt may be a placeholder. */
export function isRoutable(e: AgentSpawnInput): boolean {
  return !e.fork && !e.resume && !e.workflow
}

async function isEnabled($: EngineInterface, options: PluginOptions): Promise<boolean> {
  if (options.enabled === true) return true
  try {
    return (await $.env.get('DOWNSHIFT_MOD_ROUTER')) === '1'
  } catch {
    return false
  }
}

async function readOptional(fn: () => Promise<string>): Promise<string | undefined> {
  try {
    const v = await fn()
    return typeof v === 'string' && v !== '' ? v : undefined
  } catch {
    return undefined
  }
}

/** One JSON line per spawn to DOWNSHIFT_MOD_ROUTER_LOG, when set: for `claude -p`, where nothing draws. Best effort. */
async function appendTrace($: EngineInterface, record: Record<string, unknown>): Promise<void> {
  let path: string | undefined
  try {
    path = await $.env.get('DOWNSHIFT_MOD_ROUTER_LOG')
  } catch {
    return
  }
  if (!path) return
  try {
    let previous = ''
    try {
      previous = await $.fs.read(path)
    } catch {
      previous = ''
    }
    await $.fs.write(path, previous + JSON.stringify({ ts: new Date().toISOString(), ...record }) + '\n')
  } catch {
    // tracing never changes the route
  }
}

export const register: Register = (on, options) => {
  const binary = typeof options.binary === 'string' && options.binary !== '' ? options.binary : 'downshift'

  on('agent.spawn', async ($, e, next) => {
    if (!(await isEnabled($, options))) return next(e)
    if (!isRoutable(e)) return next(e)

    const sessionId = await readOptional(() => $.session.id())
    const cwd = e.cwd ?? (await readOptional(() => $.session.cwd()))
    const payload = buildPayload(e, sessionId, cwd)
    const requested = e.model ?? ''
    const startedAt = Date.now()

    let model: string | null = null
    let note = ''
    try {
      const run = await $.process.run([binary, 'claude-code'], { stdin: JSON.stringify(payload), timeoutMs: RUN_TIMEOUT_MS })
      if (run.exitCode !== 0) {
        note = `exit ${run.exitCode}`
      } else {
        model = modelFromHookOutput(run.stdout)
        if (model === null) note = 'no model in hook output'
      }
      const stderrLine = run.stderr.split('\n').find(l => l.includes('correlation_id=')) ?? ''
      if (stderrLine) note = note ? `${note}; ${stderrLine.trim()}` : stderrLine.trim()
    } catch (err) {
      note = `binary failed: ${err instanceof Error ? err.message : String(err)}`
    }

    const elapsedMs = Date.now() - startedAt
    const changed = model !== null && model !== requested
    const trace = {
      event: 'agent.spawn', tool_use_id: e.tool_use_id, subagent_type: e.subagentType, requested_model: requested || null,
      parent_model: e.parentModel, written_model: changed ? model : null, elapsed_ms: elapsedMs, note,
    }
    await appendTrace($, trace)

    if (!changed) {
      if (note && model === null) $.ui.log(`${LOG_PREFIX}: spawn ${e.tool_use_id} left unchanged (${note})`, { to: 'debug' })
      return next(e)
    }
    $.ui.log(`${LOG_PREFIX}: ${e.subagentType || 'agent'} ${requested || e.parentModel} → ${model} (${elapsedMs} ms)`)
    return next({ ...e, model: model as string })
  })
}
