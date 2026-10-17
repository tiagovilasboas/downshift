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
// Phase 2b also listens for `turn.complete` on subagent loops: when the agent
// has finished, accumulated API usage is handed to `downshift end claude-code`
// (same ChildEnd recorder as SubagentStop) so pricing no longer needs the
// child transcript. `turn.step` effort steering is still open.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type {
  AgentSpawnInput,
  EngineInterface,
  PluginOptions,
  Register,
  TurnCompleteInput,
  TurnUsage,
} from 'claude-code'

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

/** SubagentStop-shaped payload `downshift end claude-code` accepts with inline usage. */
export type StopPayload = {
  hook_event_name: 'SubagentStop'
  agent_id: string
  session_id?: string
  model?: string
  entry?: 'mod'
  usage: {
    input_tokens: number
    output_tokens: number
    cache_read_input_tokens: number
    cache_creation_input_tokens: number
  }
}

type AccUsage = {
  input_tokens: number
  output_tokens: number
  cache_read_input_tokens: number
  cache_creation_input_tokens: number
  model: string
  session_id?: string
  flushed: boolean
}

const DONE_STATUSES = new Set(['completed', 'failed', 'killed'])

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

/** Builds the stop payload the shared end runner prices from inline API usage. */
export function buildStopPayload(agentId: string, acc: AccUsage): StopPayload {
  const payload: StopPayload = {
    hook_event_name: 'SubagentStop',
    agent_id: agentId,
    entry: 'mod',
    usage: {
      input_tokens: acc.input_tokens,
      output_tokens: acc.output_tokens,
      cache_read_input_tokens: acc.cache_read_input_tokens,
      cache_creation_input_tokens: acc.cache_creation_input_tokens,
    },
  }
  if (acc.session_id) payload.session_id = acc.session_id
  if (acc.model) payload.model = acc.model
  return payload
}

/** Adds one turn's usage into the running total for that agent. */
export function accumulateUsage(prev: AccUsage | undefined, usage: TurnUsage, sessionId: string | undefined): AccUsage {
  const next: AccUsage = prev
    ? { ...prev }
    : {
        input_tokens: 0,
        output_tokens: 0,
        cache_read_input_tokens: 0,
        cache_creation_input_tokens: 0,
        model: '',
        flushed: false,
      }
  next.input_tokens += usage.input_tokens || 0
  next.output_tokens += usage.output_tokens || 0
  next.cache_read_input_tokens += usage.cache_read_input_tokens || 0
  next.cache_creation_input_tokens += usage.cache_creation_input_tokens || 0
  if (usage.model) next.model = usage.model
  if (sessionId) next.session_id = sessionId
  return next
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

/** True when the agent loop is finished (or gone). False when still running or list is unavailable. */
export async function agentIsTerminal($: EngineInterface, agentId: string): Promise<boolean> {
  try {
    const list = await $.agent.list()
    const row = list.find(a => a.id === agentId)
    if (!row) return true
    return DONE_STATUSES.has(row.status)
  } catch {
    // Without list we cannot know multi-turn is done; leave usage for SubagentStop.
    return false
  }
}

export const register: Register = (on, options) => {
  const binary = typeof options.binary === 'string' && options.binary !== '' ? options.binary : 'downshift'
  /** Per-agent token totals across turn.complete events until the loop ends. */
  const usageByAgent = new Map<string, AccUsage>()

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

  on('turn.complete', async ($, e: TurnCompleteInput, next) => {
    const result = await next(e)
    if (!(await isEnabled($, options))) return result
    if (!e.agentId || !e.usage) return result

    const sessionId = await readOptional(() => $.session.id())
    const acc = accumulateUsage(usageByAgent.get(e.agentId), e.usage, sessionId)
    usageByAgent.set(e.agentId, acc)

    if (acc.flushed) return result
    if (!(await agentIsTerminal($, e.agentId))) {
      await appendTrace($, {
        event: 'turn.complete', agent_id: e.agentId, note: 'accumulated, agent still running',
        input_tokens: acc.input_tokens, output_tokens: acc.output_tokens,
      })
      return result
    }

    const payload = buildStopPayload(e.agentId, acc)
    let note = ''
    try {
      const run = await $.process.run([binary, 'end', 'claude-code'], {
        stdin: JSON.stringify(payload),
        timeoutMs: RUN_TIMEOUT_MS,
      })
      if (run.exitCode !== 0) note = `exit ${run.exitCode}`
      else note = (run.stderr.split('\n').find(l => l.trim() !== '') ?? '').trim() || 'usage sent'
    } catch (err) {
      note = `binary failed: ${err instanceof Error ? err.message : String(err)}`
    }
    acc.flushed = true
    usageByAgent.set(e.agentId, acc)
    await appendTrace($, {
      event: 'turn.complete', agent_id: e.agentId, model: acc.model || null,
      input_tokens: acc.input_tokens, output_tokens: acc.output_tokens, note,
    })
    if (note && !note.includes('usage')) {
      $.ui.log(`${LOG_PREFIX}: turn.complete ${e.agentId} (${note})`, { to: 'debug' })
    }
    return result
  })
}
