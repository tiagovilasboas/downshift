// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The agent sprite: stable per id, a tool per kind of agent, one pose per
// state, still under reduced motion, a free-standing figure with no frame.

import { expect, test } from 'claude-code/testing'

import { activityOf, agentSprite, stateOf } from './agent-sprite'
import { HUE } from '../shapes'
import { ACTIVITIES } from './idle'
import { headset, LEAD_ORANGE, orchestratorSprite } from './orchestrator'
import { TOOL_PAINT } from './paint'
import { poseOf } from './poses'
import type { SpriteState } from './poses'
import { kindOf, TOOLS } from './tools'
import type { Kind } from './tools'

const STATUSES = ['running', 'pending', 'completed', 'failed', 'blocked', 'cancelled']
const STATES: SpriteState[] = ['work', 'idle', 'done', 'failed']
const KINDS = Object.keys(TOOLS) as Kind[]
const frontIsClear = (g: string[][]) => g.slice(5, 10).every(row => row.slice(3, 9).every(c => ['#', 'S', 'k', '.'].includes(c)))
const rects = (s: string) => [...s.matchAll(/<rect x="(\d+)" y="(\d+)" width="(\d+)" height="(\d+)"\/>/g)].map(m => m.slice(1).map(Number))

test('the same id draws the same sprite, byte for byte, and ids spread over every idle activity', () => {
  expect(agentSprite('toolu_1', 'cancelled', 'green', 24, 'coder').source).toBe(agentSprite('toolu_1', 'cancelled', 'green', 24, 'coder').source)
  const picks = new Set(Array.from({ length: 300 }, (_, i) => activityOf(`toolu_${i}`)))
  expect(picks.size).toBe(ACTIVITIES.length)
})

test('the agent type names its kind by whole words, and an unknown type holds no tool', () => {
  expect(kindOf('downshift:coder')).toBe('coder')
  expect(kindOf('downshift:coder-back')).toBe('coder-back')
  expect(kindOf('downshift:coder-front')).toBe('coder-front')
  expect(kindOf('Explore')).toBe('researcher')
  expect(kindOf('senior-staff-qa')).toBe('tester')
  expect(kindOf('senior-staff-appsec')).toBe('security')
  expect(kindOf('downshift:mechanical-worker')).toBe('mechanical')
  expect(kindOf('Plan')).toBe('architect')
  expect(kindOf('senior-staff-ai-finops')).toBe('ai')
  expect(kindOf('senior-staff-fullstack-implementer')).toBe('coder')
  expect(kindOf('senior-staff-backend-platform')).toBe('platform')
  expect(kindOf('senior-staff-writer')).toBe('documenter')
  for (const k of ['refactor', 'reviewer', 'debugger', 'security', 'researcher', 'documenter', 'architect', 'platform', 'ai', 'performance', 'tester']) {
    expect(kindOf(`downshift:${k}`)).toBe(k)
  }
  expect(kindOf('maintainer')).toBeUndefined()
  expect(kindOf('general-purpose')).toBeUndefined()
  expect(kindOf(undefined)).toBeUndefined()
})

test('every kind holds a tool of its own, and the tool changes the drawing', () => {
  const arts = new Set(KINDS.map(k => TOOLS[k].art.join('/')))
  expect(arts.size).toBe(KINDS.length)
  const bare = agentSprite('a', 'running', HUE.orange, 24, 'general-purpose').source
  for (const k of KINDS) {
    const s = agentSprite('a', 'running', HUE.orange, 24, `downshift:${k}`).source
    expect(s).not.toBe(bare)
    expect(Object.values(TOOL_PAINT).some(c => s.includes(`fill="${c}"`))).toBe(true)
  }
})

test('the status picks the state; work and idle animate, done and failed stand still with their bubble', () => {
  expect(STATUSES.map(stateOf)).toEqual(['work', 'idle', 'done', 'failed', 'idle', 'idle'])
  const work = agentSprite('a', 'running', 'orange', 24, 'coder').source
  expect(work).toContain('class="f1"')
  expect(work).toContain('0.9s steps(1,end) infinite')
  for (let i = 0; i < ACTIVITIES.length; i++) {
    const loop = poseOf('idle', 'coder', i)
    expect(loop.frames.length).toBeGreaterThan(1)
    expect(loop.period).toBeGreaterThan(poseOf('work', 'coder').period)
  }
  const done = agentSprite('a', 'completed', 'orange').source
  const failed = agentSprite('a', 'failed', 'orange').source
  for (const still of [done, failed]) expect(still).not.toContain('@keyframes')
  expect(done).toContain('#639922')
  expect(failed).toContain('#E24B4A')
})

test('a failed figure slumps one cell onto its legs: only the feet row stays', () => {
  const top = (s: SpriteState) => Math.min(...poseOf(s, undefined).frames[0]!.map((row, y) => (row.includes('#') ? y : 99)))
  expect(top('failed')).toBe(top('done') + 1)
  const row = (s: SpriteState, y: number) => poseOf(s, undefined).frames[0]![y]!.join('')
  expect(row('failed', 11)).toBe(row('done', 11))
  expect(row('failed', 10)).toBe(row('done', 9).replace(/#/g, 'S'))
})

test('nothing covers the front of the body: tools and props stay beside it or above it', () => {
  for (const s of STATES) {
    for (const k of [...KINDS, undefined]) {
      for (let a = 0; a < ACTIVITIES.length; a++) {
        for (const g of poseOf(s, k, a).frames) expect(frontIsClear(g)).toBe(true)
      }
    }
  }
  for (const speaking of [false, true]) expect(frontIsClear(headset(speaking, speaking))).toBe(true)
})

test('every state switches its animation off under reduced motion, and the first frame shows without it', () => {
  let animated = 0
  for (const status of STATUSES) {
    const s = agentSprite('id-2', status, 'green', 24, 'tester').source
    if (!s.includes('class="f1"')) continue
    animated++
    expect(s).toContain('@media (prefers-reduced-motion: reduce){.f0,.f1')
    const hidden = /<style>([^{]*)\{opacity:0\}/.exec(s)?.[1] ?? ''
    expect(hidden.split(',')).toContain('.f1')
    expect(hidden.split(',')).not.toContain('.f0')
  }
  expect(animated).toBeGreaterThanOrEqual(2)
})

test('the figure stands free: no frame, no rounded rect, no stroke, every rect on the grid, at the size asked', () => {
  for (const status of STATUSES) {
    for (const type of [...KINDS, 'general-purpose']) {
      for (let i = 0; i < 6; i++) {
        const d = agentSprite(`id-${i}`, status, 'green', 24, type)
        expect(d.source).not.toContain('rx=')
        expect(d.source).not.toContain('stroke')
        const all = rects(d.source)
        expect(all.length).toBeGreaterThan(10)
        expect(all.every(([x, y, w, h]) => h === 1 && w! < 12 && x! + w! <= 12 && y! < 12)).toBe(true)
        expect([d.width, d.height]).toEqual([24, 24])
        expect(d.source).toContain('viewBox="0 0 12 12"')
        expect(d.source.length).toBeLessThan(131072)
      }
    }
  }
})

test('the orchestrator wears a headset, points and speaks while active, and stands still when idle', () => {
  const on = orchestratorSprite(true).source
  const off = orchestratorSprite(false).source
  expect(on).toContain('fill="#444441"')
  expect(on).toContain('class="f1"')
  expect(on).toContain('fill="#EF9F27"')
  expect(off).not.toContain('@keyframes')
  expect(off).toContain('fill="#444441"')
  expect(orchestratorSprite(true, 30).width).toBe(30)
})

test('every agent wears the one orange, and the active orchestrator a stronger one', () => {
  expect(agentSprite('x', 'failed', 'orange').source).toContain(`<g fill="${HUE.orange}"><rect`)
  expect(orchestratorSprite(true).source).toContain(`<g fill="${LEAD_ORANGE}"><rect`)
  expect(orchestratorSprite(false).source).not.toContain(LEAD_ORANGE)
  expect(LEAD_ORANGE).not.toBe(HUE.orange)
})
