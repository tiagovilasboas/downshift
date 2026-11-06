// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The animated progress drawings: deterministic, advancing, still where asked.

import { expect, test } from 'claude-code/testing'

import { progressBar, shineBar } from './progress'
import { BAR_PAD, bar } from './shapes'
import type { ProgressShare } from './progress'

const share = (over: Partial<ProgressShare> = {}): ProgressShare => ({ total: 4, completed: 2, running: 0, finished: false, ...over })
const dotCount = (s: string) => (s.match(/<rect class="p\d"/g) ?? []).length
/** x of every dot in the fill layer (the first group). */
const fillXs = (s: string) => {
  const group = /<g fill="#22c55e">(.*?)<\/g>/.exec(s)?.[1] ?? ''
  return [...group.matchAll(/ x="([\d.]+)"/g)].map(m => Number(m[1]))
}

test('the same share draws the same bar, byte for byte', () => {
  expect(progressBar(300, 12, share()).source).toBe(progressBar(300, 12, share()).source)
})

test('dots thicken toward the head of the fill', () => {
  const xs = fillXs(progressBar(400, 12, share({ total: 1, completed: 1, finished: false })).source)
  const mid = 200
  const tail = xs.filter(x => x < mid / 2).length
  const head = xs.filter(x => x >= mid * 1.5).length
  expect(head).toBeGreaterThan(tail)
  expect(Math.max(...xs)).toBeLessThan(400)
})

test('the faint layer exists only while work is running', () => {
  const faint = (s: string) => /opacity="0\.45">(.*?)<\/g>/.exec(s)?.[1] ?? ''
  expect(faint(progressBar(300, 12, share({ running: 0 })).source)).toBe('')
  expect((faint(progressBar(300, 12, share({ running: 2 })).source).match(/<rect/g) ?? []).length).toBeGreaterThan(0)
})

test('a tick marks each remaining step beyond the fill', () => {
  const ticks = (s: string) => (/opacity="0\.6">(.*?)<\/g>/.exec(s)?.[1].match(/<rect/g) ?? []).length
  expect(ticks(progressBar(300, 12, share({ total: 5, completed: 1 })).source)).toBe(3)
  expect(ticks(progressBar(300, 12, share({ total: 5, completed: 5, finished: true })).source)).toBe(0)
})

test('motion is off under reduced motion, and a finished bar glows slowly', () => {
  const working = progressBar(300, 12, share()).source
  const done = progressBar(300, 12, share({ completed: 4, finished: true })).source
  expect(working).toContain('@media (prefers-reduced-motion: reduce){.p0,.p1,.p2,.p3{animation:none}}')
  expect(working).toContain('50%{opacity:0.3}')
  expect(done).toContain('50%{opacity:0.8}')
  expect(done).toContain('3.2s')
  expect(working).not.toContain('3.8s')
})

test('no total or no started work draws the track alone', () => {
  for (const s of [share({ total: 0 }), share({ completed: 0 }), share({ total: NaN })]) {
    const d = progressBar(200, 12, s)
    expect(dotCount(d.source)).toBe(0)
    expect(d.source).not.toContain('<style')
  }
  expect(progressBar(200, 12, share({ completed: 0, running: 1 })).source).toContain('class="p')
})

test('the shine sweeps only over a filled bar and stops under reduced motion', () => {
  expect(shineBar(64, 6, undefined).source).not.toContain('class="sh"')
  expect(shineBar(64, 6, 0).source).not.toContain('class="sh"')
  const s = shineBar(64, 6, 50).source
  expect(s).toContain('class="sh"')
  expect(s).toContain('prefers-reduced-motion: reduce){.sh{animation:none;display:none}}')
  expect(shineBar(64, 6, 50).width).toBe(64)
})

test('every bar has 2 px of padding above and below: the Svg is 4 px taller and the bar starts at y=2', () => {
  expect(BAR_PAD).toBe(2)
  const bars = [
    ['progressBar', progressBar(200, 12, share()), 12],
    ['progressBar track', progressBar(200, 12, share({ completed: 0 })), 12],
    ['shineBar', shineBar(64, 6, 50), 6],
    ['shineBar track', shineBar(64, 6, undefined), 6],
    ['bar', bar(200, 8, 40), 8],
    ['bar track', bar(200, 8, undefined), 8],
  ] as const
  for (const [what, d, h] of bars) {
    expect([what, d.height]).toEqual([what, h + 4])
    expect(d.source).toContain(`height="${h + 4}" viewBox="0 0 ${d.width} ${h + 4}"`)
    expect(d.source).toContain('<g transform="translate(0 2)">')
  }
})
