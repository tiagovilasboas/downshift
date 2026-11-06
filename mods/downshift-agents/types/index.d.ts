// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The state contract of downshift-agents: what the mod keeps in $.state for
// the session. Labels and counts only: never a prompt, a tool output body or
// a secret.

/** Where one subagent run stands, from engine events only. */
/** `stale`: still open, but silent past the sweep threshold; any later event revives or closes it. */
export type AgentStatus = 'running' | 'stale' | 'completed' | 'failed' | 'cancelled'

/** Tokens the engine reported for an agent's turns (measured, not estimated). */
export type TokenCounts = {
  input: number
  output: number
  cacheRead: number
  cacheWrite: number
}

/** One subagent of the session, keyed by the engine's agent id. */
export type Agent = {
  id: string
  /** The Agent call's tool_use_id: joins Downshift's routing decision. */
  toolUseId?: string
  /** Downshift's hashed key of toolUseId (telemetry.ToolUseKey). */
  toolUseKey?: string
  /** The Agent call's short description (a few words), never the prompt. */
  label: string
  type: string
  /** Set only when agent.spawn named the parent loop; never inferred. */
  parentId?: string
  /** The model the spawn asked for; undefined means inherit. */
  requestedModel?: string
  parentModel?: string
  /** What the engine resolved at spawn time: a decision, not evidence. */
  resolvedModel?: string
  /** The model that answered, from turn.complete usage: the evidence. */
  effectiveModel?: string
  status: AgentStatus
  startedAt: number
  /** The last event heard for this agent; the sweep's clock. Falls back to startedAt. */
  touchedAt?: number
  endedAt?: number
  runs: number
  tools: number
  lastTool?: string
  tokens?: TokenCounts
  /**
   * True while only the agent's own events were heard, before its spawn
   * settled (or for an engine fork no spawn names): kept, never drawn.
   */
  isOrphan?: boolean
  /** Turn ids already counted, bounded, so a repeated event adds nothing. */
  seenTurns: string[]
}

export type TaskStatus =
  | 'pending'
  | 'running'
  | 'stale'
  | 'completed'
  | 'failed'
  | 'blocked'
  | 'cancelled'

export type Task = {
  id: string
  subject: string
  status: TaskStatus
  blockedBy: string[]
  /** When this task last changed status or subject; the sweep's clock. */
  touchedAt?: number
}

/** Task progress from one source: the task tools, the desktop plan, or TodoWrite. */
export type TaskBoard = {
  source: 'tasks' | 'plan' | 'todos' | 'none'
  tasks: Task[]
}

/** One routing decision Downshift logged for a spawn of this session. */
export type RoutingSpawn = {
  toolUseKey: string
  state: string
  requested: string
  final: string
  observed?: string
  label: string
  reason?: string
}

/** One command family's Compactor figures, as Downshift ranks them (most tokens avoided first). */
export type CompactorFamily = {
  family: string
  commands: number
  compacted: number
  rawTokens: number
  avoidedTokens: number
}

/** Runs with no session tag (a direct `downshift compactor -- ...`), counted apart from the session's. */
export type CompactorDirect = {
  /** Downshift's own window label, e.g. "last 30 days". */
  scope: string
  commands: number
  savedBytes: number
  estimatedTokens: number
  /** Downshift's own sentence, e.g. "direct calls, no session: 3, ~1.2k tokens saved". */
  line: string
}

export type CompactorFigures = {
  /** Downshift's own window label, e.g. "last 30 days". */
  scope: string
  commands: number
  compacted: number
  rawBytes: number
  savedBytes: number
  estimatedTokens: number
  /** Downshift's own method text, e.g. "estimated: saved_bytes / 4". */
  method: string
  /** At most five, best first; empty when the binary predates them. */
  families: CompactorFamily[]
  /** Absent when the session has no direct calls, or the binary predates them. */
  direct?: CompactorDirect
}

export type RoutingFigures = {
  switched: number
  modelSwitched: number
  effortChanged: number
  held: number
  observed: number
  /** Spawns placed against the session model. */
  children: number
  /** Of those, the ones that ran below the session model. */
  belowChildren: number
  /** Billable tokens (input, output, cache write) of those spawns; cache read is never in it. */
  billable: number
  /** Billable tokens that ran below the session model. */
  belowBillable: number
  /** Of the below-session spawns, the ones whose decision was an applied Downshift rewrite. */
  rewrittenSpawns: number
  /** Billable tokens of those rewritten spawns. */
  rewrittenBillable: number
  line: string
  spawns: RoutingSpawn[]
}

/**
 * One harness's billable tokens below the session model (Downshift's routing tokens).
 * Never summed across harnesses. The fallback fields are spawns placed against the
 * requested model only (no session model): a different baseline, never added to the others.
 */
export type BelowSessionRow = {
  harness: string
  /** False when no spawn had a known session model: shown as "unobserved", never 0. */
  observed: boolean
  spawns: number
  belowSpawns: number
  billable: number
  belowBillable: number
  rewrittenSpawns: number
  rewrittenBillable: number
  fallbackSpawns: number
  fallbackBillable: number
  fallbackBelowBillable: number
  unobservedSpawns: number
  cacheRead: number
}

/** Spawns of one harness by the tier each ran on (Downshift's counts). */
export type TierCounts = { small: number; mid: number; frontier: number; unknown: number }

/**
 * One harness's Efficiency row, as `downshift report --json` prints it
 * (internal/accounting/efficiency.go). Counts and shares only: no tokens, no
 * money, never summed across harnesses.
 */
export type EfficiencyRow = {
  harness: string
  spawns: number
  byTier: TierCounts
  ended: number
  completed: number
  reworked: number
  /** 0-1 fraction of ended spawns that were redone. */
  reworkRatio: number
  /** False when no completed task has a known tier: shown as "unobserved", never 0%. */
  premiumObserved: boolean
  /** 0-100 share of tier-known completed tasks that ran below frontier. */
  premiumPct: number
  tierKnown: number
  nonFrontier: number
  tierUnknown: number
}

/** The last read of Downshift's session panel, or why there is none. */
export type DownshiftSnapshot =
  | { status: 'unknown' }
  | { status: 'unavailable'; reason: string; at: number }
  | {
      status: 'ok'
      at: number
      routing: RoutingFigures
      compactor?: CompactorFigures
      /** What the report's token estimate covers, e.g. "last 30 days, all sessions". */
      tokensSavedScope: string
      belowSession: BelowSessionRow[]
      efficiency: EfficiencyRow[]
    }

/** Where a gliding figure started and when (ms): the views ease from `from` to the live value. */
export type Glide = { from: number; at: number }

/** The figures that glide: the task share (0..1) and the Compactor's percent cut. */
export type Glides = Partial<Record<'tasks' | 'compactor', Glide>>

export type View = {
  isExpanded: boolean
  filter: 'active' | 'all'
  isBandHidden: boolean
}

declare module 'claude-code' {
  interface PluginState {
    'downshift-agents': {
      agents: Shaped<Agent[]>
      board: Shaped<TaskBoard>
      downshift: Shaped<DownshiftSnapshot>
      view: Shaped<View>
      glides: Shaped<Glides>
    }
  }
}
