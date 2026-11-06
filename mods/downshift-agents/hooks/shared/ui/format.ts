// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Text formatting for the views: meters, counts, bytes, durations and model
// names. Presentation only; no figure is computed here.

/**
 * Downshift's palette: monochrome with one green. Green marks what is live or
 * measured good; done work is plain text; Claude's orange warns, red fails.
 */
export const ACCENT = 'success'
export const DONE = 'text'
export const WARN = 'claude'
export const FAIL = 'error'
/** Secondary text: labels, separators, a meter's empty track. */
export const MUTED = 'subtle'

/**
 * A drawn meter of `cells` cells: the filled run, a half cell when the share
 * falls between two, and the empty track. Half cells make a slow share move
 * every 1/(2·cells) instead of jumping a whole cell. An unknown share is all
 * track.
 */
export function meter(cells: number, percent: number | undefined): { fill: string; track: string } {
  const n = Math.max(1, cells)
  if (percent === undefined) {
    return { fill: '', track: '━'.repeat(n) }
  }
  const halves = Math.round((Math.min(100, Math.max(0, percent)) / 100) * n * 2)
  const full = Math.floor(halves / 2)
  const fill = '━'.repeat(full) + (halves % 2 === 1 ? '╸' : '')
  return { fill, track: '━'.repeat(n - full - (halves % 2)) }
}

export function tokens(n: number): string {
  if (n >= 1_000_000) {
    return `${(n / 1_000_000).toFixed(1)}M`
  }
  if (n >= 1_000) {
    return `${(n / 1_000).toFixed(1)}k`
  }
  return String(n)
}

export function bytes(n: number): string {
  if (n >= 1024 * 1024) {
    return `${(n / (1024 * 1024)).toFixed(1)} MB`
  }
  if (n >= 1024) {
    return `${(n / 1024).toFixed(1)} KB`
  }
  return `${n} B`
}

export function elapsed(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  if (s < 60) {
    return `${s}s`
  }
  const m = Math.floor(s / 60)
  return m < 60 ? `${m}m ${String(s % 60).padStart(2, '0')}s` : `${Math.floor(m / 60)}h ${m % 60}m`
}

/** `claude-sonnet-5-5` reads `sonnet-5-5`; an alias stays as given. */
export function modelName(id: string | undefined): string {
  if (id === undefined || id === '') {
    return 'unknown'
  }
  return id.replace(/^claude-/, '').replace(/-\d{8}$/, '')
}

/** Cuts text to `width` cells with an ellipsis. */
export function fit(text: string, width: number): string {
  if (width <= 1) {
    return ''
  }
  return text.length <= width ? text : `${text.slice(0, width - 1)}…`
}
