// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Downshift's mark, as the downshift-site draws it: two light steps and a
// green one. `brandMark` sits on its own dark rounded square, which reads on
// any pane; `bareMark` is the steps alone, light on a dark pane and dark on a
// light one.

import { HUE, INK, svgDoc } from './shapes'
import type { Drawing } from './shapes'

const STEPS = [
  'M28 22H104L168 86V140L137 171V98L94 55H28Z',
  'M28 80H81L120 119V164L94 190V130L74 110H28Z',
]
const LAST_STEP = 'M28 138H65L103 176L78 201L55 178H28Z'

/** The steps' box in the site's 200 x 220 drawing. */
const MARK = { x: 28, y: 22, width: 140, height: 179 }
const LIGHT = '#F9FAFB'

/** The pane the bare mark sits on; `auto` follows the OS scheme inside the drawing. */
export type Ground = 'dark' | 'light' | 'auto'

/** The two light steps' fill: a color, or a class the drawing's own style sets. */
function stepFill(ground: Ground): { attr: string; style: string } {
  if (ground === 'auto') {
    return {
      attr: 'class="step"',
      style: `<style>.step{fill:${INK}}@media (prefers-color-scheme: dark){.step{fill:${LIGHT}}}</style>`,
    }
  }
  return { attr: `fill="${ground === 'dark' ? LIGHT : INK}"`, style: '' }
}

/**
 * The steps alone, `size` px tall, with no tile: light on a dark ground, dark on a light one.
 * `insetLeft` px of empty space sit before the steps, inside the drawing.
 */
export function bareMark(size: number, ground: Ground, insetLeft = 0): Drawing {
  const scale = size / MARK.height
  const width = Math.ceil(MARK.width * scale) + insetLeft
  const fill = stepFill(ground)
  const steps = STEPS.map(d => `<path d="${d}" ${fill.attr}/>`).join('') + `<path d="${LAST_STEP}" fill="${HUE.green}"/>`
  const body = fill.style + `<g transform="translate(${insetLeft} 0) scale(${scale.toFixed(4)}) translate(${-MARK.x} ${-MARK.y})">${steps}</g>`
  return { source: svgDoc(width, size, body), width, height: size }
}

/** The mark on a dark rounded square, `size` px. */
export function brandMark(size = 26): Drawing {
  const scale = (size * 0.64) / MARK.height
  const x = (size - MARK.width * scale) / 2
  const y = (size - MARK.height * scale) / 2
  const steps = STEPS.map(d => `<path d="${d}" fill="${LIGHT}"/>`).join('') + `<path d="${LAST_STEP}" fill="${HUE.green}"/>`
  const body =
    `<rect x="0.5" y="0.5" width="${size - 1}" height="${size - 1}" rx="${size * 0.24}" fill="${INK}" stroke="${HUE.gray}" stroke-opacity="0.35"/>` +
    `<g transform="translate(${x.toFixed(2)} ${y.toFixed(2)}) scale(${scale.toFixed(4)}) translate(${-MARK.x} ${-MARK.y})">${steps}</g>`
  return { source: svgDoc(size, size, body), width: size, height: size }
}
