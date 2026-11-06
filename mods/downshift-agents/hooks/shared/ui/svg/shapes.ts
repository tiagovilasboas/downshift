// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Svg drawings for the surfaces that draw Svg (desktop, vscode, mobile): a
// rounded progress bar and a pill. Each returns its markup and the size to
// draw it at. An Svg is drawn as an image and cannot follow the theme, so the
// hues are mid-tones and the track is translucent: both read on a light pane
// and on a dark one.

export type Drawing = { source: string; width: number; height: number }

/**
 * One hue per meaning, the same as the theme keys of the text, from the
 * downshift-site palette (its green, its foreground and its muted gray), plus
 * Claude's orange for warnings.
 */
export const HUE = {
  green: '#22c55e',
  light: '#d6dae2',
  orange: '#d97757',
  red: '#ef4444',
  gray: '#868a90',
} as const
export type Hue = keyof typeof HUE

/** A near-black ground: a hue on it, or it on a hue, keeps its contrast on either theme. */
export const INK = '#0A0A0A'

export const TRACK = 'rgba(134,138,144,0.28)'
const FONT = "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"

/** Class of an element that breathes while its work runs; `pulseStyle` animates it. */
export const PULSE_CLASS = 'pu'

/** A gentle blink (opacity 1 to 0.45, 1.8 s) for `.pu`, switched off under reduced motion. */
export const pulseStyle =
  `<style>.${PULSE_CLASS}{animation:pulse 1.8s ease-in-out infinite}@keyframes pulse{0%,100%{opacity:1}50%{opacity:0.45}}` +
  `@media (prefers-reduced-motion: reduce){.${PULSE_CLASS}{animation:none}}</style>`

export function escapeXml(text: string): string {
  return text.replace(/[&<>"']/g, c => `&#${c.charCodeAt(0)};`)
}

/** The markup around a body; `stretch` lets a drawn width other than its own stretch it. */
export function svgDoc(width: number, height: number, body: string, stretch = false): string {
  const aspect = stretch ? ' preserveAspectRatio="none"' : ''
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"${aspect}>${body}</svg>`
}

/** Empty px above and below every bar inside its drawing, so a surface never clips its ends. */
export const BAR_PAD = 2

/** A bar drawing: `body` (drawn `height` tall from y=0) inset by the padding; the Svg is `height + 2 * BAR_PAD` tall. */
export function barDoc(width: number, height: number, body: string): Drawing {
  const total = height + BAR_PAD * 2
  return { source: svgDoc(width, total, `<g transform="translate(0 ${BAR_PAD})">${body}</g>`, true), width, height: total }
}

/** A one-pixel dim rule that stretches to its slot: the divider between list rows. */
export function rule(): Drawing {
  return { source: svgDoc(100, 1, `<rect width="100" height="1" fill="${TRACK}"/>`, true), width: 100, height: 1 }
}

/**
 * A rounded bar `width` px wide. An unknown share draws the track alone; a
 * share above zero always shows a sliver, so started work never reads as none.
 */
export function bar(width: number, height: number, percent: number | undefined, hue: Hue = 'green'): Drawing {
  const w = Math.max(height * 2, Number.isFinite(width) ? Math.round(width) : 0)
  const r = height / 2
  const track = `<rect width="${w}" height="${height}" rx="${r}" fill="${TRACK}"/>`
  // NaN fails `> 0` too: an unreadable share draws the track alone
  if (percent === undefined || !(percent > 0)) {
    return barDoc(w, height, track)
  }
  const fill = Math.max(height, Math.round((Math.min(100, percent) / 100) * w))
  const shade =
    `<defs><linearGradient id="f" x1="0" x2="1" y1="0" y2="0">` +
    `<stop offset="0" stop-color="${HUE[hue]}" stop-opacity="0.75"/><stop offset="1" stop-color="${HUE[hue]}"/>` +
    `</linearGradient></defs>`
  const body = `${shade}${track}<rect width="${fill}" height="${height}" rx="${r}" fill="url(#f)"/>`
  return barDoc(w, height, body)
}

/**
 * The width text takes at `size` px in the system face, glyph by glyph.
 * The pill draws its text at exactly this length, so it always fits.
 */
export function textWidth(text: string, size: number): number {
  let em = 0
  for (const c of text) {
    em += emOf(c)
  }
  return Math.ceil(em * size)
}

/** One glyph's width in em, by class: one width for every letter stretches `LIVE` and squeezes `WAITING`. */
function emOf(c: string): number {
  if (/[ .,:;'|!Iijl]/.test(c)) return 0.28
  if (/[frt1]/.test(c)) return 0.4
  if (/[mwMW%]/.test(c)) return 0.86
  if (/[A-Z]/.test(c)) return 0.64
  if (/[a-z0-9]/.test(c)) return 0.58
  return 0.45
}

/**
 * A rounded pill with a dot and a label in `hue` on a dark ground tinted in
 * it: the label keeps its contrast on a light pane too. `md` is the header's
 * LIVE pill; `sm` a status chip. `pulse` breathes the dot where the surface
 * animates an Svg; elsewhere it stands still.
 */
export function pill(label: string, hue: Hue, opts: { size?: 'sm' | 'md'; pulse?: boolean } = {}): Drawing {
  const isMd = opts.size === 'md'
  const font = isMd ? 12 : 11
  const height = isMd ? 24 : 20
  const pad = isMd ? 10 : 8
  const dot = isMd ? 4 : 3
  const text = textWidth(label, font)
  const width = pad + dot * 2 + 6 + text + pad
  const color = HUE[hue]
  const pulses = opts.pulse === true
  const outline = `x="0.5" y="0.5" width="${width - 1}" height="${height - 1}" rx="${(height - 1) / 2}"`
  const body =
    (pulses ? pulseStyle : '') +
    `<rect ${outline} fill="${INK}"/>` +
    `<rect ${outline} fill="${color}" fill-opacity="0.12" stroke="${color}" stroke-opacity="0.55"/>` +
    `<circle${pulses ? ` class="${PULSE_CLASS}"` : ''} cx="${pad + dot}" cy="${height / 2}" r="${dot}" fill="${color}"/>` +
    `<text x="${pad + dot * 2 + 6}" y="${height / 2 + font * 0.35}" font-family="${FONT}" font-size="${font}" ` +
    `font-weight="600" fill="${color}" textLength="${text}" lengthAdjust="spacingAndGlyphs">${escapeXml(label)}</text>`
  return { source: svgDoc(width, height, body), width, height }
}
