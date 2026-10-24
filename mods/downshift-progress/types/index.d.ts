// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// State contract of the downshift-progress mod. Type-check with the tsconfig.json
// beside this folder (`tsc -p mods/downshift-progress`). Values are added by the
// feature tasks (roster, flow, downshift data).

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

/** The slice of `session --panel` the band keeps; every figure is the binary's own. */
export type BandPanel = {
  summary: string | null
  modelSwitched: number | null
  effortChanged: number | null
  held: number | null
  /** One entry per decided spawn; `state` is the binary's word for it. */
  spawns: { state: string | null }[]
}

/** The report strings the band prints as given. */
export type BandReport = {
  tokensSaved: string | null
  tokensSavedShort: string | null
  scopeLabel: string | null
}

export type BandReportCache = { report: BandReport | null; at: number | null }

declare module 'claude-code' {
  interface PluginState {
    'downshift-progress': {
      flow: Flow | null
      flowSteps: Record<string, FlowStep>
      /** The band's last `session --panel` answer; null while none, or when the binary fails. */
      bandPanel: BandPanel | null
      /** The last report block the band saw, and when (epoch ms). */
      bandReport: BandReportCache
    }
  }
}
