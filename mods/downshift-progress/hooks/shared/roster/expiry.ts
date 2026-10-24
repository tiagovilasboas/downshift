// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Pure expiry rules of the roster: a function of (rows, now), nothing else.

import type { FinishedRun } from '../../../types'
import { FINISHED_SHOW_MS } from './constants'

/** True while a finished row is on show: it expires exactly at `FINISHED_SHOW_MS`. */
export function isShown(row: Pick<FinishedRun, 'finishedAt'>, nowMs: number): boolean {
  return nowMs - row.finishedAt < FINISHED_SHOW_MS
}

/** The rows still on show at `nowMs`, oldest first as given. */
export function shownRows(rows: readonly FinishedRun[], nowMs: number): FinishedRun[] {
  return rows.filter(row => isShown(row, nowMs))
}
