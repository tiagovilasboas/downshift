// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Svg drawings: their size, their invariants and the hygiene every markup
// must keep, since a surface draws it as an image and bounds it at 131072 chars.

import { expect, test } from 'claude-code/testing'

import type { AgentStatus, TaskStatus } from '../../../../types'
import { lookOf } from '../status'
import { bareMark, brandMark } from './brand'
import { agentSprite } from './sprite/agent-sprite'
import { icon, statusMark } from './icons'
import type { IconName } from './icons'
import { progressBar, shineBar } from './progress'
import { BAR_PAD, HUE, bar, escapeXml, pill, textWidth } from './shapes'
import type { Drawing, Hue } from './shapes'

const MAX_SOURCE = 131072

// Records, so the compiler fails this file when a status or an icon is added without a test.
const STATUS_SET: Record<TaskStatus | AgentStatus, true> = { pending: true, running: true, stale: true, completed: true, failed: true, blocked: true, cancelled: true }
const ICON_SET: Record<IconName, true> = { agents: true, layers: true, trend: true, code: true, routing: true, info: true, tasks: true, bot: true }
const STATUSES = Object.keys(STATUS_SET) as (TaskStatus | AgentStatus)[]
const ICONS = Object.keys(ICON_SET) as IconName[]
const HUES = Object.keys(HUE) as Hue[]
const SHARES = [undefined, 0, 1, 33, 57, 99, 100, 140, -5]
const LABELS = ['LIVE', 'WAITING', 'N/A', 'running', 'done', 'failed', 'blocked', 'cancelled', 'pending']

const attr = (tag: string, name: string) => new RegExp(`\\s${name}="([^"]*)"`).exec(tag)?.[1]
const rootOf = (d: Drawing) => /^<svg\b[^>]*>/.exec(d.source)?.[0] ?? ''
/** Widths of the `<rect width=... height=...>` of a bar: the track first, then the fill when drawn. */
const barRects = (d: Drawing) => [...d.source.matchAll(/<rect width="([^"]+)" height=/g)].map(m => Number(m[1]))
const textOf = (d: Drawing) => /<text\b[^>]*>/.exec(d.source)?.[0] ?? ''

/** Every drawing the mod can make, labelled by the call that made it. */
function everyDrawing(): [string, Drawing][] {
  const all: [string, Drawing][] = []
  for (const hue of HUES) {
    for (const share of SHARES) for (const width of [4, 120, 333]) all.push([`bar(${width},8,${share},${hue})`, bar(width, 8, share, hue)])
    for (const label of LABELS) {
      for (const size of ['sm', 'md'] as const) for (const pulse of [false, true]) all.push([`pill(${label},${hue},${size},${pulse})`, pill(label, hue, { size, pulse })])
    }
    for (const name of ICONS) {
      for (const size of [12, 16, 18, 26]) all.push([`icon(${name},${hue},${size})`, icon(name, hue, size)])
    }
    for (const status of STATUSES) for (const size of [12, 14, 16]) all.push([`statusMark(${status},${hue},${size})`, statusMark(status, hue, size)])
  }
  for (const done of [0, 1, 3, 4]) for (const running of [0, 2]) all.push([`progressBar(${done},${running})`, progressBar(240, 12, { total: 4, completed: done, running, finished: done === 4 })])
  for (const share of SHARES) all.push([`shineBar(${share})`, shineBar(64, 6, share)])
  for (let i = 0; i < 8; i++) for (const status of ['running', 'pending', 'completed', 'failed'] as const) all.push([`agentSprite(${i},${status})`, agentSprite(`id-${i}`, status, 'green', 24)])
  for (const size of [16, 26, 30, 40]) all.push([`brandMark(${size})`, brandMark(size)])
  for (const ground of ['dark', 'light', 'auto'] as const) for (const size of [14, 18, 26]) all.push([`bareMark(${size},${ground})`, bareMark(size, ground)])
  return all
}

/** Every `<` opens a well-formed tag with double-quoted attributes, and every tag closes in order. */
function wellFormed(source: string): boolean {
  const tag = /<(\/?)([A-Za-z][\w:-]*)((?:\s+[\w:-]+="[^"<>]*")*)\s*(\/?)>/g
  const stack: string[] = []
  let tags = 0
  for (const m of source.matchAll(tag)) {
    tags++
    if (m[4] === '/') continue
    if (m[1] === '/') {
      if (stack.pop() !== m[2]) return false
    } else stack.push(m[2] ?? '')
  }
  return stack.length === 0 && tags === (source.match(/</g) ?? []).length && !/&(?!#\d+;)/.test(source)
}

test('a bar keeps its width at every share, in its markup and in its size', () => {
  const bad: string[] = []
  for (const share of SHARES) {
    for (const width of [40, 120, 333]) {
      const d = bar(width, 8, share)
      const ok = d.width === width && d.height === 8 + 2 * BAR_PAD && attr(rootOf(d), 'width') === String(width) && barRects(d)[0] === width
      if (!ok) bad.push(`bar(${width},8,${share}) -> ${d.width}x${d.height}, track ${barRects(d)[0]}`)
    }
  }
  expect(bad).toEqual([])
  // Narrower than two heights, it takes two heights, so the track still reads as a bar.
  expect(bar(5, 8, 50).width).toBe(16)
  expect(bar(0, 8, undefined).width).toBe(16)
})

test('an unknown or empty share draws the track alone, a started one always shows a sliver', () => {
  for (const share of [undefined, 0, -5]) expect(barRects(bar(200, 8, share))).toEqual([200])
  expect(barRects(bar(200, 8, 1))).toEqual([200, 8])
  expect(barRects(bar(200, 8, 57))).toEqual([200, 114])
  expect(barRects(bar(200, 8, 100))).toEqual([200, 200])
  expect(barRects(bar(200, 8, 140))).toEqual([200, 200])
})

test('a bar fill never exceeds the track and never shrinks as the share grows', () => {
  const bad: string[] = []
  const boxes: [number, number][] = [[1, 4], [16, 8], [40, 10], [200, 8], [333, 10]]
  for (const [width, height] of boxes) {
    let last = 0
    for (let share = -10; share <= 150; share++) {
      const [track = 0, fill = 0] = barRects(bar(width, height, share))
      if (fill > track || fill < last) bad.push(`bar(${width},${height},${share}): track ${track}, fill ${fill}, previous ${last}`)
      last = fill
    }
  }
  expect(bad).toEqual([])
})

test('a bar draws a number a surface can read, even for a share or width that is not finite', () => {
  const bad = [
    bar(200, 8, NaN),
    bar(200, 8, Infinity),
    bar(200, 8, -Infinity),
    bar(NaN, 8, 50),
    bar(Infinity, 8, 50),
    bar(200, 8, 50, 'green'),
  ].filter(d => /NaN|Infinity|undefined/.test(d.source) || !Number.isFinite(d.width))
  expect(bad.map(d => d.source)).toEqual([])
})

test('a pill grows with its label and its text always fits inside it', () => {
  for (const size of ['sm', 'md'] as const) {
    const widths = ['a', 'ab', 'abc', 'abcd', 'abcde'].map(l => pill(l, 'green', { size }).width)
    expect(widths.every((w, i) => i === 0 || w > (widths[i - 1] ?? Infinity))).toBe(true)
  }
  const bad: string[] = []
  for (const label of LABELS) {
    for (const size of ['sm', 'md'] as const) {
      const d = pill(label, 'orange', { size })
      const t = textOf(d)
      const fits = Number(attr(t, 'x')) + Number(attr(t, 'textLength')) < d.width
      if (!fits) bad.push(`${label}/${size}: text ends past ${d.width}`)
    }
  }
  expect(bad).toEqual([])
  // The chrome around the text is the same for every label of one size.
  const chrome = (label: string, size: 'sm' | 'md') => {
    const d = pill(label, 'green', { size })
    return d.width - Number(attr(textOf(d), 'textLength'))
  }
  expect(new Set(LABELS.map(l => chrome(l, 'md'))).size).toBe(1)
  expect(new Set(LABELS.map(l => chrome(l, 'sm'))).size).toBe(1)
  expect(pill('LIVE', 'green', { size: 'md' }).height).toBeGreaterThan(pill('LIVE', 'green').height)
})

test('a pill sets its text at the length it measured, so the font cannot overflow it', () => {
  for (const [size, font] of [['sm', 11], ['md', 12]] as const) {
    const t = textOf(pill('WAITING', 'gray', { size }))
    expect(attr(t, 'font-size')).toBe(String(font))
    expect(attr(t, 'textLength')).toBe(String(textWidth('WAITING', font)))
    expect(attr(t, 'lengthAdjust')).toBe('spacingAndGlyphs')
  }
  expect(textWidth('', 12)).toBe(0)
  expect(textWidth('LIVE', 24)).toBeGreaterThan(textWidth('LIVE', 12))
  expect(textWidth('LIVE', 12)).toBeGreaterThan(textWidth('live', 12))
})

test('a pill escapes its label and only pulses when asked', () => {
  const d = pill('a<b&"c\'', 'red')
  expect(d.source).toContain('>a&#60;b&#38;&#34;c&#39;</text>')
  expect(d.source).not.toContain('a<b')
  expect(wellFormed(pill('</text><script>alert(1)</script>', 'red').source)).toBe(true)
  expect(pill('</text><script>x</script>', 'red').source).not.toContain('<script')
  expect(escapeXml(`&<>"'`)).toBe('&#38;&#60;&#62;&#34;&#39;')
  expect(pill('LIVE', 'green', { pulse: true }).source).toContain('class="pu"')
  expect(pill('LIVE', 'green').source).not.toContain('pulse')
})

test('icons and marks are drawn at the size asked, in the box they declare', () => {
  const bad: string[] = []
  const check = (what: string, d: Drawing, size: number) => {
    const root = rootOf(d)
    const ok = d.width === size && d.height === size && attr(root, 'width') === String(size) && attr(root, 'height') === String(size) && attr(root, 'viewBox') === `0 0 ${size} ${size}`
    if (!ok) bad.push(`${what}: ${d.width}x${d.height} ${root}`)
  }
  for (const name of ICONS) {
    for (const size of [10, 12, 16, 18, 26, 40]) check(`icon(${name},${size})`, icon(name, 'green', size), size)
    expect(icon(name, 'green').width).toBe(16)
  }
  for (const status of STATUSES) {
    for (const size of [12, 14, 16, 24]) check(`statusMark(${status},${size})`, statusMark(status, 'green', size), size)
    expect(statusMark(status, 'green').width).toBe(14)
  }
  expect(bad).toEqual([])
})

test('the brand mark is a dark tile at the size asked, with its steps inside it, and no theme to follow', () => {
  const bad: string[] = []
  for (const size of [14, 16, 26, 32, 40]) {
    const d = brandMark(size)
    const root = rootOf(d)
    if (d.width !== size || d.height !== size || attr(root, 'viewBox') !== `0 0 ${size} ${size}`) bad.push(`brandMark(${size}): ${d.width}x${d.height} ${root}`)
    // The tile carries its own ground, so it reads on a light pane and a dark one.
    if (!/<rect x="0.5" y="0.5"[^>]*fill="#0A0A0A"/.test(d.source)) bad.push(`brandMark(${size}): no dark tile`)
    // The steps sit inside the tile: the transform scales them down and centers them.
    const m = /translate\(([-\d.]+) ([-\d.]+)\) scale\(([\d.]+)\)/.exec(d.source)
    const [x, y, k] = [Number(m?.[1]), Number(m?.[2]), Number(m?.[3])]
    if (!(x > 0 && y > 0 && k > 0 && x + 140 * k < size && y + 179 * k < size)) bad.push(`brandMark(${size}): steps leave the tile (${x}, ${y}, x${k})`)
  }
  expect(bad).toEqual([])
  expect(brandMark().width).toBe(26)
  expect(brandMark().source).toContain(HUE.green)
})

test('each icon is its own drawing, in the hue asked and no other', () => {
  expect(new Set(ICONS.map(n => icon(n, 'green').source)).size).toBe(ICONS.length)
  expect(new Set(HUES.map(h => icon('agents', h).source)).size).toBe(HUES.length)
  for (const hue of HUES) {
    const others = HUES.filter(h => h !== hue).map(h => HUE[h])
    for (const d of [icon('bot', hue), statusMark('failed', hue)]) {
      expect(d.source).toContain(HUE[hue])
      expect(others.some(o => d.source.includes(o))).toBe(false)
    }
  }
})

test('every drawing is a clean, closed, script-less Svg under the size cap', () => {
  const drawings = everyDrawing()
  expect(drawings.length).toBeGreaterThan(500)
  const bad: string[] = []
  for (const [what, d] of drawings) {
    const s = d.source
    const problems = [
      !s.startsWith('<svg ') && 'does not start with <svg',
      !s.endsWith('</svg>') && 'does not end with </svg>',
      !wellFormed(s) && 'not well formed',
      /<script/i.test(s) && 'has a script',
      /\son\w+\s*=/i.test(s) && 'has an event-handler attribute',
      /href\s*=|<image|<foreignObject|<use\b|@import|javascript:/i.test(s) && 'reaches outside itself',
      /url\((?!#)/i.test(s) && 'has an external url()',
      s.replace('http://www.w3.org/2000/svg', '').includes('http') && 'names a host beyond the namespace',
      /NaN|Infinity|undefined|\[object/.test(s) && 'has a bad number or value',
      s.length >= MAX_SOURCE && `is ${s.length} chars`,
      !(d.width > 0 && d.height > 0) && 'has no size',
    ].filter(Boolean)
    if (problems.length > 0) bad.push(`${what}: ${problems.join(', ')}`)
  }
  expect(bad).toEqual([])
})

test('every url(#id) points at an id in the same document, and an id is declared once', () => {
  const bad: string[] = []
  for (const [what, d] of everyDrawing()) {
    const ids = [...d.source.matchAll(/\sid="([^"]+)"/g)].map(m => m[1])
    const refs = [...d.source.matchAll(/url\(#([^)]+)\)/g)].map(m => m[1])
    if (new Set(ids).size !== ids.length || refs.some(r => !ids.includes(r))) bad.push(`${what}: ids ${ids}, refs ${refs}`)
  }
  expect(bad).toEqual([])
})

test('only a bar stretches to the width a surface gives it, so no glyph is distorted', () => {
  for (const [what, d] of everyDrawing()) {
    const stretches = rootOf(d).includes('preserveAspectRatio="none"')
    expect([what, stretches]).toEqual([what, /^(bar|progressBar|shineBar)\(/.test(what)])
  }
})

test('every status has a mark in its own hue, and the text color and the hue agree', () => {
  const marks = STATUSES.map(s => ({ s, look: lookOf(s) }))
  const bad: string[] = []
  for (const { s, look } of marks) {
    const d = statusMark(s, look.hue)
    if (!HUES.includes(look.hue)) bad.push(`${s}: unknown hue ${look.hue}`)
    if (!d.source.includes(HUE[look.hue])) bad.push(`${s}: mark is not drawn in ${look.hue}`)
    if (!/<g transform="[^"]+"><(circle|path)/.test(d.source)) bad.push(`${s}: empty mark`)
  }
  expect(bad).toEqual([])
  // Every status has its own distinct mark, even in a single hue.
  expect(new Set(STATUSES.map(s => statusMark(s, 'green').source)).size).toBe(STATUSES.length)
  // One theme color, one hue: the same meaning reads the same as text and as an Svg.
  const hueByColor = new Map<string, Set<Hue>>()
  for (const { look } of marks) hueByColor.set(look.color, (hueByColor.get(look.color) ?? new Set()).add(look.hue))
  expect([...hueByColor.values()].every(h => h.size === 1)).toBe(true)
  expect(new Set([...hueByColor.values()].map(h => [...h][0])).size).toBe(hueByColor.size)
})

test('only a running mark turns', () => {
  for (const s of STATUSES) {
    const turns = statusMark(s, 'green').source.includes('<animateTransform')
    expect([s, turns]).toEqual([s, s === 'running'])
  }
})

test('the bare mark is the steps alone, light on a dark ground, dark on a light one, the OS scheme on auto', () => {
  for (const size of [14, 18, 26]) {
    for (const ground of ['dark', 'light', 'auto'] as const) {
      const d = bareMark(size, ground)
      expect(d.height).toBe(size)
      expect(d.width).toBeLessThan(size)
      expect(d.source).not.toContain('<rect')
      expect(d.source).toContain(HUE.green)
    }
  }
  // an inset widens the drawing by exactly its px and shifts the steps right
  expect(bareMark(18, 'dark', 2).width).toBe(bareMark(18, 'dark').width + 2)
  expect(bareMark(18, 'dark', 2).source).toContain('translate(2 0)')
  expect(bareMark(18, 'dark', 2).height).toBe(18)
  expect(bareMark(18, 'dark').source).toContain('fill="#F9FAFB"')
  expect(bareMark(18, 'dark').source).not.toContain('<style')
  expect(bareMark(18, 'light').source).toContain('fill="#0A0A0A"')
  expect(bareMark(18, 'auto').source).toMatch(/<style>\.step\{fill:#0A0A0A\}@media \(prefers-color-scheme: dark\)\{\.step\{fill:#F9FAFB\}\}<\/style>/)
})

test('a running marker blinks gently and a done one stands still, both off under reduced motion', () => {
  const running = pill('running', 'green', { pulse: true }).source
  expect(running).toContain('animation:pulse 1.8s ease-in-out infinite')
  expect(running).toContain('50%{opacity:0.45}')
  expect(running).toContain('prefers-reduced-motion: reduce){.pu{animation:none}}')
  expect(pill('done', 'light').source).not.toContain('animation')
})
