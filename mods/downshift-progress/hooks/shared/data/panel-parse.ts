// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Pure parser of the `session --panel` JSON. Unknown fields are ignored, missing
// or mistyped fields become null, and garbage answers null (never throws).

import type {
  Panel,
  PanelCompactor,
  PanelDisplay,
  PanelReport,
  PanelScope,
  PanelSpawn,
} from './panel'

type Json = Record<string, unknown>

const isObject = (v: unknown): v is Json =>
  typeof v === 'object' && v !== null && !Array.isArray(v)

const num = (v: unknown): number | null =>
  typeof v === 'number' && Number.isFinite(v) ? v : null

const str = (v: unknown): string | null => (typeof v === 'string' ? v : null)

/** A string that is empty means "nothing to say", the same as absent. */
const text = (v: unknown): string | null => {
  const s = str(v)
  return s === null || s === '' ? null : s
}

const parseSpawn = (raw: Json): PanelSpawn => ({
  toolUse: text(raw.tool_use),
  state: text(raw.state),
  label: text(raw.label),
  fromModel: text(raw.from_model),
  toModel: text(raw.to_model),
  observedModel: text(raw.observed_model),
  savedTotal: num(raw.saved_total),
  savedShort: text(raw.saved_short),
})

const parseScope = (raw: unknown): PanelScope => {
  const s = isObject(raw) ? raw : {}
  return { kind: text(s.kind), days: num(s.days), label: text(s.label) }
}

const parseDisplay = (raw: unknown): PanelDisplay => {
  const d = isObject(raw) ? raw : {}
  return {
    tokensSaved: text(d.tokens_saved),
    tokensSavedShort: text(d.tokens_saved_short),
    scopeLabel: text(d.scope_label),
    routing: text(d.routing),
    compactor: text(d.compactor),
  }
}

const parseCompactor = (raw: unknown): PanelCompactor | null => {
  if (!isObject(raw)) return null
  return {
    commands: num(raw.commands),
    compacted: num(raw.compacted),
    savedBytes: num(raw.saved_bytes),
    tokens: num(raw.tokens),
  }
}

const parseReport = (raw: Json): PanelReport => ({
  tokensSavedEstimate: num(raw.tokens_saved_estimate),
  scope: parseScope(raw.scope),
  display: parseDisplay(raw.display),
  compactor: parseCompactor(raw.compactor),
})

/**
 * Parses the JSON of `downshift session --panel`. Returns null when the text is
 * not a JSON object carrying a session or a spawns list.
 */
export function parsePanel(json: string): Panel | null {
  let raw: unknown
  try {
    raw = JSON.parse(json)
  } catch {
    return null
  }
  if (!isObject(raw)) return null
  if (typeof raw.session !== 'string' && !Array.isArray(raw.spawns)) return null
  const spawns = Array.isArray(raw.spawns) ? raw.spawns.filter(isObject) : []
  return {
    session: text(raw.session),
    switched: num(raw.switched),
    modelSwitched: num(raw.model_switched),
    effortChanged: num(raw.effort_changed),
    held: num(raw.held),
    summary: text(raw.summary),
    spawns: spawns.map(parseSpawn),
    report: isObject(raw.report) ? parseReport(raw.report) : null,
  }
}
