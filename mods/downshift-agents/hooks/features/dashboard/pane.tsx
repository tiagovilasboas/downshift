// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The agents dashboard: a pane docked beside the transcript (or inline above
// the prompt where the surface has no dock). It reads state only; the one I/O
// it can start is the Refresh button's Downshift read, off the render path.
// Top to bottom: what the session is doing (tasks, agents), then what Downshift
// measured (Compactor, routing, efficiency), then the controls. A surface that draws Svg
// gets the graphic panels; the terminal gets the glyph cards.

import type { EngineInterface, On } from 'claude-code'

import type { Agent, DownshiftSnapshot, TaskBoard, View } from '../../../types'
import { refresh } from '../downshift/refresh'
import { shownCut } from '../metrics/cut'
import { visibleAgents } from '../metrics/normalize'
import { progressOf } from '../tasks/board'
import { shownProgress } from '../tasks/shown'
import type { ShownProgress } from '../tasks/shown'
import { readAll, updateView } from '../../shared/state/atoms'
import { ACCENT, MUTED } from '../../shared/ui/format'
import { groundOfSettings } from '../../shared/ui/ground'
import type { Ground } from '../../shared/ui/svg/brand'
import { agentsSection } from './agents-section'
import { agentsPanel } from './desktop/agents'
import type { GraphicUi } from './desktop/frame'
import { efficiencyPanel } from './desktop/efficiency'
import { compactorPanel, routingPanel } from './desktop/metrics'
import { header, tiles } from './desktop/overview'
import { tasksPanel } from './desktop/tasks'
import { efficiencySection } from './efficiency-section'
import { ESTIMATE_NOTE, compactorSection, routingSection } from './metrics-section'
import { tasksSection } from './tasks-section'
import type { Ui } from './ui'

export const PANE_ID = 'downshift-agents'
export const PANE_TITLE = 'Downshift Agents'

type Shown = {
  agents: readonly Agent[]
  board: TaskBoard
  snapshot: DownshiftSnapshot
  view: View
  now: number
  /** The task progress and the Compactor's percent cut as they glide. */
  tasks: ShownProgress
  cut?: number
}

/**
 * Shows the pane when a subagent starts. Unasked, it takes no focus, and a
 * narrow terminal keeps it waiting until it is wider or the person opens it.
 * An open pane is only retitled. A refused open is not an error.
 */
export async function showPane($: EngineInterface): Promise<void> {
  try {
    await $.ui.open({ id: PANE_ID, title: PANE_TITLE })
  } catch {
    // a ui.open hook may refuse; the band still shows the agents
  }
}

export function drawPane(on: On): void {
  on('ui.render', { component: 'Pane', requestId: PANE_ID }, async ($, e) => {
    const ui = $.ui.resolve(e)
    const { agents, board, downshift: snapshot, view, glides } = await readAll($)
    const now = await $.clock.now()
    const tasks = shownProgress(progressOf(board), glides.tasks, now)
    const shown: Shown = { agents, board, snapshot, view, now, tasks, cut: shownCut(snapshot, glides.compactor, now) }
    const columns = Math.max(20, e.props.bodyColumns)
    const isGraphic = e.surface === 'desktop' || e.surface === 'vscode' || e.surface === 'mobile'
    const { Box, Text } = ui

    return (
      <Box flexDirection="column" gap={isGraphic ? 'sm' : 0}>
        {isGraphic
          ? graphicBody($.ui.resolve(e), shown, columns, await groundOfSettings($))
          : glyphBody(ui, shown, columns)}
        {controls($, ui, view)}
        <Text color={MUTED} size="caption" wrap="wrap">
          {ESTIMATE_NOTE}
        </Text>
      </Box>
    )
  })
}

/** The terminal's tree: glyph meters and framed text cards. */
function glyphBody(ui: Ui, s: Shown, columns: number) {
  const { Box, Text } = ui
  const isLive = s.snapshot.status === 'ok'
  return [
    <Box flexDirection="row" justifyContent="space-between" gap={1} paddingX={1}>
      <Text bold wrap="truncate-end">
        <Text color={ACCENT}>◆ </Text>
        AGENTS · LIVE
      </Text>
      <Text color={isLive ? ACCENT : MUTED}>{isLive ? '● Downshift connected' : '○ Downshift metrics unavailable'}</Text>
    </Box>,
    tasksSection(ui, s.board, s.tasks, columns, s.view.isExpanded),
    agentsSection(ui, { agents: s.agents, snapshot: s.snapshot, filter: s.view.filter, isExpanded: s.view.isExpanded, now: s.now }, columns),
    compactorSection(ui, s.snapshot, s.cut, columns),
    routingSection(ui, s.snapshot, visibleAgents(s.agents).length, columns),
    efficiencySection(ui, s.snapshot, columns),
  ]
}

/** The tree for a surface that draws Svg and type sizes: header, task progress, tiles, panels. */
function graphicBody(ui: GraphicUi, s: Shown, columns: number, ground: Ground) {
  const isExpanded = s.view.isExpanded
  return [
    header(ui, s.snapshot, ground),
    tasksPanel(ui, s.board, s.tasks, columns, isExpanded),
    tiles(ui, s.agents, s.snapshot, columns),
    agentsPanel(ui, { agents: s.agents, snapshot: s.snapshot, filter: s.view.filter, isExpanded, now: s.now }),
    compactorPanel(ui, s.snapshot, s.cut, columns, isExpanded),
    routingPanel(ui, s.snapshot, visibleAgents(s.agents).length, isExpanded),
    efficiencyPanel(ui, s.snapshot, isExpanded),
  ]
}

/**
 * The pane's buttons. `Button` takes no fill, border or colour prop (only
 * `variant`, `plain`, `dimColor` and `hover`), and a desktop draws its own native
 * button either way, so an outline button cannot be built. What a Button does
 * take is `Text` children, so the label is drawn in the accent hue, on the
 * default `secondary` look (never the filled `primary`); the `label` keeps the
 * control's accessible name.
 */
function controls($: EngineInterface, ui: Ui, view: View) {
  const { Box, Button, Text } = ui
  const tint = (text: string) => <Text color={ACCENT}>{text}</Text>
  const filter = view.filter === 'active' ? 'All agents' : 'Active only'
  const expand = view.isExpanded ? 'Less' : 'Details'
  const band = view.isBandHidden ? 'Show band' : 'Hide band'
  return (
    <Box flexDirection="row" gap={1} flexWrap="wrap" marginTop={1}>
      <Button key="filter" label={filter} variant="secondary" onPress={() => updateView($, v => ({ ...v, filter: v.filter === 'active' ? 'all' : 'active' }))}>
        {tint(filter)}
      </Button>
      <Button key="expand" label={expand} variant="secondary" onPress={() => updateView($, v => ({ ...v, isExpanded: !v.isExpanded }))}>
        {tint(expand)}
      </Button>
      <Button key="refresh" label="Refresh" variant="secondary" onPress={() => void refresh($, true)}>
        {tint('Refresh')}
      </Button>
      <Button key="band" label={band} variant="secondary" onPress={() => updateView($, v => ({ ...v, isBandHidden: !v.isBandHidden }))}>
        {tint(band)}
      </Button>
    </Box>
  )
}
