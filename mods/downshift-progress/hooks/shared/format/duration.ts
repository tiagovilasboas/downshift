// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Time only: these helpers never touch token or cost numbers.

const pad2 = (n: number): string => String(n).padStart(2, '0')

/** Whole seconds of a millisecond span; anything not positive and finite is 0. */
function wholeSeconds(ms: number): number {
  return Number.isFinite(ms) && ms > 0 ? Math.floor(ms / 1000) : 0
}

/** `12s`, `3m 05s`, `1h 02m`. Seconds are dropped once an hour is reached. */
export function formatDuration(ms: number): string {
  const total = wholeSeconds(ms)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  if (hours > 0) return `${hours}h ${pad2(minutes)}m`
  if (minutes > 0) return `${minutes}m ${pad2(seconds)}s`
  return `${seconds}s`
}

/** Time since `startedAt` (epoch ms) at `now`; a clock that went backwards reads `0s`. */
export function elapsedSince(startedAt: number, now: number): string {
  return formatDuration(now - startedAt)
}
