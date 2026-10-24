// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Types of the `downshift session --panel` answer as the mod reads it. Every
// field may be absent on an older binary, so a missing value is null and the UI
// shows what exists. Nothing here is computed: the Go side owns every figure.

/** One subagent decision of the session, as the binary reports it. */
export type PanelSpawn = {
  /** Hashed harness tool call id (sha256 hex): the join key to a roster row. */
  toolUse: string | null
  state: string | null
  label: string | null
  fromModel: string | null
  toModel: string | null
  observedModel: string | null
  /** Work plus cache tokens saved, summed by the binary; null on older binaries. */
  savedTotal: number | null
  /** The same figure in the CLI token format, e.g. "58k". */
  savedShort: string | null
}

/** The window the report covers. */
export type PanelScope = {
  kind: string | null
  days: number | null
  label: string | null
}

/** Shell-output compression over the report window. */
export type PanelCompactor = {
  commands: number | null
  compacted: number | null
  savedBytes: number | null
  tokens: number | null
}

/** Ready-to-print text of the report figures. */
export type PanelDisplay = {
  tokensSaved: string | null
  tokensSavedShort: string | null
  scopeLabel: string | null
  routing: string | null
  compactor: string | null
}

/** The accounting block: the figures of `downshift report` over the window. */
export type PanelReport = {
  tokensSavedEstimate: number | null
  scope: PanelScope
  display: PanelDisplay
  compactor: PanelCompactor | null
}

/** One session's panel. `report` is null when the answer carries none. */
export type Panel = {
  session: string | null
  switched: number | null
  modelSwitched: number | null
  effortChanged: number | null
  held: number | null
  /** The binary's one-line summary; null when it has nothing true to say. */
  summary: string | null
  spawns: PanelSpawn[]
  report: PanelReport | null
}

/** Why the panel is not available now; the UI draws what it already has. */
export type PanelFailure =
  | 'not-installed'
  | 'no-session'
  | 'timeout'
  | 'bad-json'
  | 'unavailable-surface'

export type PanelResult = { ok: true; panel: Panel } | { ok: false; reason: PanelFailure }
