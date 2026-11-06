// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Self-animating progress drawings. The motion lives inside the Svg (CSS
// keyframes), so it plays without the engine redrawing; reduced-motion turns
// it off. `progressBar` is the dashboard's dithered bar; `shineBar` is the
// band's thin bar, too slim to dither, with a highlight sweeping over its fill.

import { HUE, TRACK, barDoc } from './shapes'
import type { Drawing, Hue } from './shapes'

export type ProgressShare = { total: number; completed: number; running: number; finished: boolean }

/** One height for every progress bar the views draw: the band's, the task progress's and the Compactor's. */
export const BAR_HEIGHT = 8

const CELL = 3
const DOT = 2
/** Fill density at the start of the bar and at its head. */
const DENSITY_TAIL = 0.35
const DENSITY_HEAD = 0.95
/** Density of the faint layer: work running, not yet completed. */
const FAINT_DENSITY = 0.2
const FAINT_OPACITY = 0.45
/** Ticks closer than this many px are noise, so they are not drawn. */
const MIN_TICK_SPACING = 6

/** Twinkle period (s) and negative delay (s) of each of the four dot classes, working and finished. */
const WORKING = { dur: [2.2, 2.8, 1.9, 3.3], floor: 0.3 }
const FINISHED = { dur: [3.2, 3.8, 4.4, 3.5], floor: 0.8 }
const DELAY = [0, -0.7, -1.3, -0.4]

/** A stable value in [0, 1) from a cell and a salt: the same cell always draws the same, so nothing reshuffles. */
function noise(x: number, y: number, salt: number): number {
  let h = Math.imul(x + 1, 374761393) ^ Math.imul(y + 1, 668265263) ^ Math.imul(salt + 1, 2246822519)
  h = Math.imul(h ^ (h >>> 13), 1274126177)
  h ^= h >>> 16
  return (h >>> 0) / 4294967296
}

const px = (n: number): number => (Number.isFinite(n) ? Math.max(0, Math.round(n)) : 0)

function twinkleStyle(finished: boolean): string {
  const t = finished ? FINISHED : WORKING
  const rules = [0, 1, 2, 3].map(i => `.p${i}{animation:twinkle ${t.dur[i]}s ease-in-out ${DELAY[i]}s infinite}`).join('')
  const off = '.p0,.p1,.p2,.p3{animation:none}'
  return (
    `<style>${rules}@keyframes twinkle{0%,100%{opacity:1}50%{opacity:${t.floor}}}` +
    `@media (prefers-reduced-motion: reduce){${off}}</style>`
  )
}

function dots(cols: [number, number], rows: number, top: number, size: number, density: (c: number) => number, salt: number): string {
  let out = ''
  for (let c = cols[0]; c < cols[1]; c++) {
    const d = density(c)
    for (let r = 0; r < rows; r++) {
      if (noise(c, r, salt) < d) {
        const cls = Math.floor(noise(r, c, salt + 1) * 4)
        out += `<rect class="p${cls}" x="${c * CELL + 0.5}" y="${top + r * CELL + 0.5}" width="${size}" height="${size}"/>`
      }
    }
  }
  return out
}

/**
 * A rounded bar `width` px wide: dithered squares that thicken toward the head
 * of the fill and twinkle out of phase, a faint layer for work still running,
 * and a tick at each remaining step. No started work draws the track alone.
 */
export function progressBar(width: number, height: number, share: ProgressShare, hue: Hue = 'green'): Drawing {
  const w = Math.max(height * 2, px(width))
  const r = height / 2
  const track = `<rect width="${w}" height="${height}" rx="${r}" fill="${TRACK}"/>`
  const { total, completed, running } = share
  const hasTotal = Number.isFinite(total) && total > 0
  const done = hasTotal && Number.isFinite(completed) ? Math.min(total, Math.max(0, completed)) : 0
  const live = hasTotal && Number.isFinite(running) ? Math.min(total - done, Math.max(0, running)) : 0
  if (!hasTotal || (done <= 0 && live <= 0)) return barDoc(w, height, track)

  const finished = share.finished && done >= total
  const fillW = px((w * done) / total)
  const runW = px((w * (done + live)) / total)
  const rows = Math.max(1, Math.floor(height / CELL))
  const top = (height - rows * CELL) / 2
  const cols = Math.ceil(fillW / CELL)
  const span = Math.max(1, cols - 1)
  const fill =
    dots([0, cols], rows, top, DOT, c => DENSITY_TAIL + (DENSITY_HEAD - DENSITY_TAIL) * Math.pow(c / span, 1.2), 0)
  const faint = dots([cols, Math.ceil(runW / CELL)], rows, top, DOT, () => FAINT_DENSITY, 7)

  let ticks = ''
  if (w / total >= MIN_TICK_SPACING) {
    for (let i = 1; i < total; i++) {
      const x = Math.round((w * i) / total)
      if (x > fillW + 4) ticks += `<rect x="${x}" y="${height / 2 - 2}" width="1.5" height="4" rx="0.75"/>`
    }
  }

  const color = HUE[hue]
  const body =
    twinkleStyle(finished) +
    `<defs><clipPath id="c"><rect width="${w}" height="${height}" rx="${r}"/></clipPath></defs>` +
    track +
    `<g clip-path="url(#c)"><g fill="${color}">${fill}</g>` +
    `<g fill="${color}" opacity="${FAINT_OPACITY}">${faint}</g>` +
    `<g fill="${HUE.gray}" opacity="0.6">${ticks}</g></g>`
  return barDoc(w, height, body)
}

/**
 * A thin bar like `bar()`, with a highlight that sweeps across its fill. A
 * share of nothing draws the track alone. Reduced motion hides the highlight.
 */
export function shineBar(width: number, height: number, percent: number | undefined, hue: Hue = 'green'): Drawing {
  const w = Math.max(height * 2, px(width))
  const r = height / 2
  const track = `<rect width="${w}" height="${height}" rx="${r}" fill="${TRACK}"/>`
  if (percent === undefined || !(percent > 0)) return barDoc(w, height, track)

  const fillW = Math.max(height, px((Math.min(100, percent) / 100) * w))
  const shineW = Math.max(height * 2, Math.round(fillW / 3))
  const color = HUE[hue]
  const style =
    `<style>.sh{animation:sweep 2.4s ease-in-out infinite}` +
    `@keyframes sweep{from{transform:translateX(${-shineW}px)}to{transform:translateX(${fillW}px)}}` +
    `@media (prefers-reduced-motion: reduce){.sh{animation:none;display:none}}</style>`
  const defs =
    `<defs><linearGradient id="f" x1="0" x2="1" y1="0" y2="0">` +
    `<stop offset="0" stop-color="${color}" stop-opacity="0.75"/><stop offset="1" stop-color="${color}"/></linearGradient>` +
    `<linearGradient id="s" x1="0" x2="1" y1="0" y2="0"><stop offset="0" stop-color="#fff" stop-opacity="0"/>` +
    `<stop offset="0.5" stop-color="#fff" stop-opacity="0.55"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient>` +
    `<clipPath id="c"><rect width="${fillW}" height="${height}" rx="${r}"/></clipPath></defs>`
  const body =
    style + defs + track +
    `<rect width="${fillW}" height="${height}" rx="${r}" fill="url(#f)"/>` +
    `<g clip-path="url(#c)"><rect class="sh" x="0" width="${shineW}" height="${height}" fill="url(#s)"/></g>`
  return barDoc(w, height, body)
}
