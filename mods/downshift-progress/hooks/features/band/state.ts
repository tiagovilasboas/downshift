// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The band's cached `session --panel` answer. The atoms and every update stay in
// this file: the engine's scan lists a module's writes from one file.

import { atom, read, update } from 'claude-code'
import type { StateDollar } from 'claude-code'

import type { BandPanel, BandReportCache } from '../../../types'
import { NO_BAND_REPORT } from './snapshot'

const panelAtom = atom({ plugin: 'downshift-progress', key: 'bandPanel' } as const, null)
const reportAtom = atom({ plugin: 'downshift-progress', key: 'bandReport' } as const, NO_BAND_REPORT)

export function readBandPanel($: StateDollar): Promise<BandPanel | null> {
  return read($, panelAtom)
}
export function readBandReport($: StateDollar): Promise<BandReportCache> {
  return read($, reportAtom)
}

export async function writeBand($: StateDollar, panel: BandPanel | null, report: BandReportCache): Promise<void> {
  await update($, panelAtom, () => panel)
  await update($, reportAtom, () => report)
}
