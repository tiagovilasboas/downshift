// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Line 2 of the band: what the binary reports, printed as it gives it. No sums,
// no unit conversion: the figures are the binary's *_short strings and counts.

import type { BandPanel, BandReport } from '../../../types'
import type { Lang } from '../../shared/i18n/lang'
import { t } from '../../shared/i18n/t'
import { joinGroups } from './line'
import type { Line } from './line'

const muted = (text: string): Line => [{ text, token: 'muted' }]

/** The binary's one-line summary; an older binary gives counts, printed verbatim, zero ones left out. */
function routing(panel: BandPanel, lang: Lang): Line {
  if (panel.summary) return muted(panel.summary)
  const counts: [number | null, string][] = [
    [panel.modelSwitched, t('model', lang)],
    [panel.effortChanged, t('effort', lang)],
    [panel.held, t('held', lang)],
  ]
  const parts = counts.filter(([n]) => n !== null && n > 0).map(([n, label]) => `${n} ${label}`)
  return parts.length === 0 ? [] : muted(['Downshift', ...parts].join(' · '))
}

/** "Tokens saved · <scope label>: <short figure>", from the report block as the binary printed it. */
function saved(report: BandReport | null, lang: Lang): Line {
  const figure = report?.tokensSavedShort ?? report?.tokensSaved ?? null
  if (figure === null) return []
  const scope = report?.scopeLabel ? ` · ${report.scopeLabel}` : ''
  return [{ text: `${t('tokensSaved', lang)}${scope}: ${figure}`, token: 'accent' }]
}

export function summaryLine(panel: BandPanel | null, report: BandReport | null, running: number, lang: Lang): Line {
  const live: Line = running > 0 ? [{ text: `${running} ${t('running', lang).toLowerCase()}`, token: 'text' }] : []
  return joinGroups([panel === null ? [] : routing(panel, lang), saved(report, lang), live])
}
