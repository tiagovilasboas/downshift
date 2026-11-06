// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The CSS that plays a sprite's frames from inside its Svg: each frame is a
// group shown for an equal share of the cycle, with steps() timing so the
// motion is pixel-art frames, not tweens. Without animation (reduced motion,
// or a surface that runs no CSS) only the first frame shows.

/** The class of frame `i`. */
export const frameClass = (i: number): string => `f${i}`

const pct = (n: number): string => `${Math.round(n * 1000) / 10}%`

/** The stylesheet for `count` frames over `period` seconds; none for a single frame. */
export function motionStyle(count: number, period: number): string {
  if (count < 2) return ''
  const all = Array.from({ length: count }, (_, i) => `.${frameClass(i)}`)
  let css = `${all.slice(1).join(',')}{opacity:0}`
  for (let i = 0; i < count; i++) {
    const from = i / count
    const to = (i + 1) / count
    const keys = i === 0 ? `0%{opacity:1}${pct(to)}{opacity:0}100%{opacity:0}` : `0%{opacity:0}${pct(from)}{opacity:1}${pct(to)}{opacity:0}`
    css += `.${frameClass(i)}{animation:k${i} ${period}s steps(1,end) infinite}@keyframes k${i}{${keys}}`
  }
  css += `@media (prefers-reduced-motion: reduce){${all.join(',')}{animation:none}}`
  return `<style>${css}</style>`
}
