// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Downshift agent sprite: a wide-bodied pixel figure with a tool in its left
// hand that says what kind of agent it is. Its state shows in its pose, its
// eyes and a bubble; every agent wears Claude's orange, so the tool alone tells
// them apart and the stronger orange stays the orchestrator's. A waiting agent
// plays an activity picked by a hash of its id, so it never changes between renders.

import type { AgentStatus } from '../../../../../types'
import { HUE } from '../shapes'
import type { Drawing, Hue } from '../shapes'
import { ACTIVITIES } from './idle'
import type { Loop } from './idle'
import { frameClass, motionStyle } from './motion'
import { HINT, paint } from './paint'
import type { Colours } from './paint'
import { GRID, hash } from './pixels'
import { poseOf } from './poses'
import type { SpriteState } from './poses'
import { kindOf } from './tools'

export type { SpriteState }

const INK_EYE = '#2c2c2a'
/** Eye and bubble colours by state: green when done, red when failed, ink and gray otherwise. */
const SIGNAL: Record<SpriteState, { eye: string; bubble: string }> = {
  work: { eye: INK_EYE, bubble: HINT },
  idle: { eye: INK_EYE, bubble: HINT },
  done: { eye: '#3B6D11', bubble: '#639922' },
  failed: { eye: '#A32D2D', bubble: '#E24B4A' },
}

/** A palette name or a colour as given. */
const colourOf = (hue: Hue | string): string => (hue in HUE ? HUE[hue as Hue] : hue)

/** The state an agent status plays. */
export function stateOf(status: AgentStatus | string): SpriteState {
  if (status === 'running') return 'work'
  if (status === 'completed') return 'done'
  if (status === 'failed') return 'failed'
  return 'idle'
}

/** The idle activity of an agent id; the same id always does the same one. */
export function activityOf(id: string): number {
  return hash(id) % ACTIVITIES.length
}

/** The Svg of a loop: every frame its own group, the first one shown without animation. */
export function drawLoop(loop: Loop, colours: Colours, size: number): Drawing {
  const frames = loop.frames.map((g, i) => (loop.frames.length > 1 ? `<g class="${frameClass(i)}">${paint(g, colours)}</g>` : paint(g, colours)))
  const inner = `${motionStyle(loop.frames.length, loop.period)}<g shape-rendering="crispEdges">${frames.join('')}</g>`
  const source = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${GRID} ${GRID}">${inner}</svg>`
  return { source, width: size, height: size }
}

/** The sprite of one agent: its tool by type, its activity by id, the body colour given, the pose its status plays. */
export function agentSprite(id: string, status: AgentStatus | string, body: Hue | string, size = 24, type?: string): Drawing {
  const state = stateOf(status)
  return drawLoop(poseOf(state, kindOf(type), activityOf(id)), { body: colourOf(body), ...SIGNAL[state] }, size)
}
