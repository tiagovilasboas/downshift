// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Labels only the band draws (the flow phases). The shared labels come from
// shared/i18n; scope labels and token figures come from the binary as is.

import type { FlowPhase } from '../../../types'
import type { Lang } from '../../shared/i18n/lang'

const PHASES: Readonly<Record<Lang, Readonly<Record<FlowPhase, string>>>> = {
  en: { plan: 'plan', design: 'design', delegate: 'delegate', review: 'review', close: 'close' },
  'pt-BR': { plan: 'planejar', design: 'projetar', delegate: 'delegar', review: 'revisar', close: 'fechar' },
}

export const phaseLabel = (phase: FlowPhase, lang: Lang): string => PHASES[lang][phase]
