// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Joins engine facts with Downshift's figures for the views. It picks and
// labels values; the only arithmetic is the share of two measured byte counts
// and agent counts. Downshift's own accounting is never redone here.

import type { Agent, BelowSessionRow, CompactorFigures, DownshiftSnapshot, EfficiencyRow, RoutingSpawn } from '../../../types'
import { tokens } from '../../shared/ui/format'

/** The agents a view draws: orphans (heard of, never named) stay hidden. */
export const visibleAgents = (agents: readonly Agent[]): Agent[] => agents.filter(a => a.isOrphan !== true)

export type AgentCounts = { active: number; stale: number; total: number; failed: number }

export function agentCounts(agents: readonly Agent[]): AgentCounts {
  const shown = visibleAgents(agents)
  return {
    active: shown.filter(a => a.status === 'running').length,
    stale: shown.filter(a => a.status === 'stale').length,
    total: shown.length,
    failed: shown.filter(a => a.status === 'failed').length,
  }
}

/**
 * The model to name for an agent, and how sure we are. `observed` needs
 * evidence: the engine's usage record of the agent's turn, or Downshift's own
 * observed model. Otherwise the requested or decided model is named as such.
 */
export type ModelClaim = { model: string; basis: 'observed' | 'decided' | 'requested' | 'inherited' }

export function modelClaim(agent: Agent, spawn: RoutingSpawn | undefined): ModelClaim {
  if (agent.effectiveModel !== undefined) {
    return { model: agent.effectiveModel, basis: 'observed' }
  }
  if (spawn?.observed !== undefined) {
    return { model: spawn.observed, basis: 'observed' }
  }
  if (spawn !== undefined && spawn.final !== 'unknown') {
    return { model: spawn.final, basis: 'decided' }
  }
  if (agent.requestedModel !== undefined) {
    return { model: agent.requestedModel, basis: 'requested' }
  }
  return { model: agent.parentModel ?? 'unknown', basis: 'inherited' }
}

/** Downshift's decision for an agent's spawn, by its hashed tool_use id. */
export function spawnOf(agent: Agent, snapshot: DownshiftSnapshot): RoutingSpawn | undefined {
  if (snapshot.status !== 'ok' || agent.toolUseKey === undefined) {
    return undefined
  }
  return snapshot.routing.spawns.find(s => s.toolUseKey === agent.toolUseKey)
}

/** Share of raw bytes the Compactor cut; undefined with no raw bytes. */
export function reductionPercent(c: CompactorFigures): number | undefined {
  return c.rawBytes === 0 ? undefined : Math.round((c.savedBytes / c.rawBytes) * 100)
}

/** " (40%)": the share of whole that part is; empty when whole is 0. */
const percentOf = (part: number, whole: number): string => (whole === 0 ? '' : ` (${Math.round((part / whole) * 100)}%)`)

/** Work tokens the engine measured for an agent: input plus output. */
export function workTokens(agent: Agent): number | undefined {
  return agent.tokens === undefined ? undefined : agent.tokens.input + agent.tokens.output
}

/** What an agent is working on: the spawn's short label on one line, or undefined when it has none. Never invented. */
export function taskOf(agent: Agent): string | undefined {
  const text = agent.label.replace(/\s+/g, ' ').trim()
  return text === '' ? undefined : text
}

/**
 * The orchestrator is the session that spawned the agents. Its model is the
 * parent model the latest spawn reported (undefined when none did); it is
 * active while any listed agent runs, idle otherwise.
 */
export type Orchestrator = { model: string | undefined; isActive: boolean }

export function orchestratorOf(agents: readonly Agent[]): Orchestrator {
  const shown = visibleAgents(agents)
  const latest = [...shown].reverse().find(a => a.parentModel !== undefined)
  return { model: latest?.parentModel, isActive: shown.some(a => a.status === 'running') }
}

/** The running agents' tasks for the band: the latest one with a label, and how many others run. */
export function workingNow(agents: readonly Agent[]): { task: string; others: number } | undefined {
  const running = visibleAgents(agents).filter(a => a.status === 'running')
  const latest = [...running].reverse().find(a => taskOf(a) !== undefined)
  if (latest === undefined) {
    return undefined
  }
  // +N counts the other running agents that also have a task to show
  return { task: taskOf(latest) ?? '', others: running.filter(a => a !== latest && taskOf(a) !== undefined).length }
}

export type RoutingLine = { label: string; value: string; isObserved: boolean }

/**
 * The pane's Routing rows, per harness as Downshift counted them, never summed
 * and in tokens only. Each observed harness gets: tokens on a lighter model
 * (with its agents), tokens on the session model (billable minus lighter),
 * tokens Downshift moved (omitted at 0) and its agents by tier. With several
 * harnesses the labels carry the harness name. A harness with no known session
 * model gets no row; with none measured at all, one muted line says so, never
 * 0. Spawns placed against the requested model only stay on their own labelled
 * line (a different baseline), never folded into the session one.
 */
export function routingLines(rows: readonly BelowSessionRow[], efficiency: readonly EfficiencyRow[] = []): RoutingLine[] {
  const shown = rows.filter(r => r.observed || r.fallbackSpawns > 0)
  const isMany = rows.filter(r => r.observed).length > 1
  const lines = shown.flatMap(r => {
    const name = (label: string) => (isMany ? `${r.harness} · ${label}` : label)
    const row = (label: string, value: string): RoutingLine => ({ label: name(label), value, isObserved: true })
    const out: RoutingLine[] = []
    if (r.observed) {
      out.push(row('On a lighter model', `${tokens(r.belowBillable)} tokens · ${r.belowSpawns} of ${agents(r.spawns)}`))
      out.push(row('On the session model', `${tokens(Math.max(0, r.billable - r.belowBillable))} tokens`))
      if (r.rewrittenSpawns > 0) {
        out.push(row('Moved by Downshift', `${tokens(r.rewrittenBillable)} tokens · ${agents(r.rewrittenSpawns)}`))
      }
      const tiers = tierRow(efficiency.find(e => e.harness === r.harness))
      if (tiers !== undefined) {
        out.push(row('Agents by tier · 30 days', tiers))
      }
    }
    if (r.fallbackSpawns > 0) {
      // no session model to compare with: measured against the model the agent asked for
      const share = `${tokens(r.fallbackBelowBillable)} of ${tokens(r.fallbackBillable)} tokens lighter than asked${percentOf(r.fallbackBelowBillable, r.fallbackBillable)} · ${agents(r.fallbackSpawns)}`
      out.push({ label: `${r.harness} · no session model`, value: share, isObserved: true })
    }
    return out
  })
  return lines.length > 0 ? lines : [{ label: 'Not measured yet', value: '', isObserved: false }]
}

/** "1 agent" or "3 agents". */
const agents = (n: number): string => `${n} ${n === 1 ? 'agent' : 'agents'}`

/** "small 19 · mid 192 · frontier 23": the non-empty tiers, or undefined when none ran. */
function tierRow(e: EfficiencyRow | undefined): string | undefined {
  if (e === undefined) {
    return undefined
  }
  const parts = (['small', 'mid', 'frontier'] as const).filter(t => e.byTier[t] > 0).map(t => `${t} ${e.byTier[t]}`)
  return parts.length > 0 ? parts.join(' · ') : undefined
}

/**
 * The Routing card's headline. One observed harness: the share of its subagent
 * tokens on a lighter model. Several: agent counts, never summed tokens.
 */
export function routingSummary(rows: readonly BelowSessionRow[], below: number, measured: number): string {
  const seen = rows.filter(r => r.observed && r.billable > 0)
  if (seen.length === 1) {
    return `${Math.round((seen[0].belowBillable / seen[0].billable) * 100)}% of subagent tokens ran on a lighter model`
  }
  return measured === 0 ? 'no agent measured yet' : `${below} of ${measured} agents on a lighter model`
}

/** The share of one harness's tokens that ran on a lighter model, or undefined when it has none measured. */
export function cheaperShare(rows: readonly BelowSessionRow[], harness: string): number | undefined {
  const r = rows.find(x => x.harness === harness && x.observed && x.billable > 0)
  return r === undefined ? undefined : Math.round((r.belowBillable / r.billable) * 100)
}

/** The direct-calls figures of the Compactor, or undefined when there are none. */
export function directLine(c: CompactorFigures): { label: string; value: string } | undefined {
  return c.direct === undefined ? undefined : { label: 'Direct calls', value: `${c.direct.commands} · ~${tokens(c.direct.estimatedTokens)}* tokens` }
}
