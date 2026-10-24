// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// State contract of the downshift-progress mod. Type-check with the tsconfig.json
// beside this folder (`tsc -p mods/downshift-progress`). Values are added by the
// feature tasks (roster, flow, downshift data).

/**
 * The report block of `session --panel`, as stored. The contract file imports
 * nothing, so this repeats the shape of `PanelReport` (shared/data/panel.ts); the
 * writer in shared/roster/state.ts is type-checked against it.
 */
export type ReportBlock = {
  tokensSavedEstimate: number | null
  scope: { kind: string | null; days: number | null; label: string | null }
  display: {
    tokensSaved: string | null
    tokensSavedShort: string | null
    scopeLabel: string | null
    routing: string | null
    compactor: string | null
  }
  compactor: { commands: number | null; compacted: number | null; savedBytes: number | null; tokens: number | null } | null
}

export type StoredReportCache = { report: ReportBlock | null; at: number | null }

/** Size class of a planned task, as the `progress` tool spells it. */
export type FlowTier = 'fable' | 'heavy' | 'careful' | 'medium' | 'light'

export type FlowPhase = 'plan' | 'design' | 'delegate' | 'review' | 'close'

/** One planned task of a flow; `after` holds the titles it waits for. */
export type FlowTask = { title: string; tier: FlowTier | null; after: string[] }

/** The flow the main session registered through the `progress` tool. */
export type Flow = {
  title: string
  phase: FlowPhase | null
  total: number
  done: number
  finished: boolean
  tasks: FlowTask[]
}

/** What a worker reported through `step`, keyed by its agent id. */
export type FlowStep = {
  done: number
  total: number
  note: string
  /** Title of the planned task whose title equals the agent's description. */
  task: string | null
}

/** One subagent of this session, from the moment it starts. */
export type AgentRun = {
  /** The harness tool call id; the roster key. */
  toolUse: string
  /** sha256 key of `toolUse` as `downshift session --panel` spells it. */
  key: string
  agentId: string
  description: string
  type: string
  /** Tier of the planned task whose title equals the description, if any. */
  tier: FlowTier | null
  /** The model the engine resolved at spawn. */
  model: string | null
  startedAt: number
  /** Tokens the agent used, summed by `usageTotal` (an observation, not accounting). */
  tokens: number
}

/** A run that ended; it stays visible until FINISHED_SHOW_MS after `finishedAt`. */
export type FinishedRun = AgentRun & { finishedAt: number; isFailed: boolean }

/** What `session --panel` says about one spawn; every field is the binary's own text. */
export type AgentFigures = {
  model: string | null
  state: string | null
  savedShort: string | null
}

/** The planned tasks that no run has started, and which flow the started set belongs to. */
export type StartedTasks = { flow: string | null; titles: string[] }

declare module 'claude-code' {
  interface PluginState {
    'downshift-progress': {
      flow: Flow | null
      flowSteps: Record<string, FlowStep>
      running: AgentRun[]
      finished: FinishedRun[]
      planned: FlowTask[]
      started: StartedTasks
      /** Per-spawn figures keyed by the panel's hashed tool_use. */
      figures: Record<string, AgentFigures>
      /** The last report block seen and when, for the 30 s refresh rule. */
      reportCache: StoredReportCache
    }
  }
}
