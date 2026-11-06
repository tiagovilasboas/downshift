// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The agent roster as pure functions over engine facts. Each function takes the
// list and returns the next one; none reads the clock or the engine, so the
// order a test applies events in is the only input.

import type { Agent, AgentStatus, TokenCounts } from '../../../types'

/** The most agents kept; finished ones go first when the list is full. */
export const MAX_AGENTS = 100
const MAX_SEEN_TURNS = 32

export type SpawnFact = {
  toolUseId: string
  toolUseKey?: string
  label: string
  type: string
  parentId?: string
  requestedModel?: string
  parentModel?: string
  /** Set when the spawn resumes an agent that already has an id. */
  resumeId?: string
  now: number
}

export type TurnFact = {
  agentId: string
  turnId: string
  reason: 'answer' | 'aborted' | 'refusal' | 'error'
  model?: string
  usage?: TokenCounts
  now: number
}

/** The id an agent holds until the engine names it. */
const PROVISIONAL_PREFIX = 'tool:'
export const provisionalId = (toolUseId: string): string => `${PROVISIONAL_PREFIX}${toolUseId}`

export function spawnStarted(list: readonly Agent[], fact: SpawnFact): Agent[] {
  const id = fact.resumeId ?? provisionalId(fact.toolUseId)
  const known = list.find(a => a.id === id)
  if (known !== undefined) {
    // A resume, or the same spawn heard twice: run it again, never duplicate.
    const isRepeat = (known.status === 'running' || known.status === 'stale') && known.toolUseId === fact.toolUseId
    return replace(list, {
      ...known,
      status: 'running',
      touchedAt: fact.now,
      endedAt: undefined,
      runs: isRepeat ? known.runs : known.runs + 1,
    })
  }
  const agent: Agent = {
    id,
    toolUseId: fact.toolUseId,
    toolUseKey: fact.toolUseKey,
    label: fact.label,
    type: fact.type,
    parentId: fact.parentId,
    requestedModel: fact.requestedModel,
    parentModel: fact.parentModel,
    status: 'running',
    startedAt: fact.now,
    touchedAt: fact.now,
    runs: 1,
    tools: 0,
    seenTurns: [],
  }
  return bounded([...list, agent])
}

/** The engine started the spawn: the agent takes its real id. */
export function spawnSettled(
  list: readonly Agent[],
  toolUseId: string,
  agentId: string,
  resolvedModel: string | undefined,
): Agent[] {
  const pending = list.find(a => a.id === provisionalId(toolUseId))
  const named = list.find(a => a.id === agentId)
  if (pending === undefined) {
    return named === undefined ? [...list] : replace(list, { ...named, resolvedModel })
  }
  const merged: Agent = { ...pending, ...heardOf(named), id: agentId, resolvedModel }
  return list
    .filter(a => a.id !== agentId)
    .map(a => (a.id === pending.id ? merged : a))
}

/** What an orphan heard before its spawn settled: activity, never identity. */
function heardOf(orphan: Agent | undefined): Partial<Agent> {
  if (orphan === undefined) {
    return {}
  }
  const heard: Partial<Agent> = { tools: orphan.tools, lastTool: orphan.lastTool, seenTurns: orphan.seenTurns }
  if (orphan.endedAt !== undefined) {
    Object.assign(heard, { status: orphan.status, endedAt: orphan.endedAt })
  }
  if (orphan.tokens !== undefined) {
    Object.assign(heard, { tokens: orphan.tokens, effectiveModel: orphan.effectiveModel })
  }
  return heard
}

/** An agent heard of before (or without) its spawn: kept hidden until named. */
function orphan(id: string, now: number): Agent {
  return { id, label: '', type: 'unknown', status: 'running', startedAt: now, touchedAt: now, runs: 1, tools: 0, seenTurns: [], isOrphan: true }
}

/** The spawn was refused: nothing started, so nothing is listed. */
export function spawnDenied(list: readonly Agent[], toolUseId: string): Agent[] {
  return list.filter(a => a.id !== provisionalId(toolUseId))
}

export function toolUsed(list: readonly Agent[], agentId: string, tool: string, now: number): Agent[] {
  const [agent, base] = known(list, agentId, now)
  // a tool call proves the agent is alive: a stale one runs again
  const status = agent.status === 'stale' ? 'running' : agent.status
  return replace(base, { ...agent, status, touchedAt: now, tools: agent.tools + 1, lastTool: tool })
}

function known(list: readonly Agent[], id: string, now: number): [Agent, Agent[]] {
  const agent = list.find(a => a.id === id)
  if (agent !== undefined) {
    return [agent, [...list]]
  }
  const stub = orphan(id, now)
  return [stub, bounded([...list, stub])]
}

/**
 * One turn of an agent ended. A subagent's run is one turn, so its status
 * follows the reason; a turn heard twice (same turnId) changes nothing.
 */
export function turnCompleted(list: readonly Agent[], fact: TurnFact): Agent[] {
  const [agent, base] = known(list, linkedId(list, fact.agentId), fact.now)
  if (agent.seenTurns.includes(fact.turnId)) {
    return [...list]
  }
  return replace(base, {
    ...agent,
    status: statusOf(fact.reason),
    endedAt: fact.now,
    touchedAt: fact.now,
    effectiveModel: fact.model ?? agent.effectiveModel,
    tokens: fact.usage === undefined ? agent.tokens : add(agent.tokens, fact.usage),
    seenTurns: [...agent.seenTurns, fact.turnId].slice(-MAX_SEEN_TURNS),
  })
}

/**
 * A turn under an id no spawn named closes the one provisional entry still
 * running, when exactly one is: that spawn never heard its agent id. With
 * zero or several candidates it stays unlinked, and the sweep deals with it.
 */
function linkedId(list: readonly Agent[], agentId: string): string {
  if (list.some(a => a.id === agentId)) {
    return agentId
  }
  const open = list.filter(a => a.status === 'running' && a.id.startsWith(PROVISIONAL_PREFIX))
  return open.length === 1 && open[0] !== undefined ? open[0].id : agentId
}

function statusOf(reason: TurnFact['reason']): AgentStatus {
  if (reason === 'answer') {
    return 'completed'
  }
  return reason === 'aborted' ? 'cancelled' : 'failed'
}

function add(a: TokenCounts | undefined, b: TokenCounts): TokenCounts {
  return {
    input: (a?.input ?? 0) + b.input,
    output: (a?.output ?? 0) + b.output,
    cacheRead: (a?.cacheRead ?? 0) + b.cacheRead,
    cacheWrite: (a?.cacheWrite ?? 0) + b.cacheWrite,
  }
}

function replace(list: readonly Agent[], next: Agent): Agent[] {
  return list.map(a => (a.id === next.id ? next : a))
}

function bounded(list: Agent[]): Agent[] {
  if (list.length <= MAX_AGENTS) {
    return list
  }
  const firstOf = (test: (a: Agent) => boolean) => list.findIndex(test)
  const pick = [firstOf(a => a.isOrphan === true), firstOf(a => a.status !== 'running' && a.status !== 'stale')].find(i => i !== -1)
  const drop = pick ?? 0
  return list.filter((_, i) => i !== drop)
}
