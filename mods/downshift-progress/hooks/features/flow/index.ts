// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Flow feature: the progress/step tools and the state the band and pane read.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { On } from 'claude-code'
import { registerFlowTools } from './tools'

export { flowPercent, readFlow, readFlowSteps, readStepOf } from './state'
export { PROGRESS_TOOL, STEP_TOOL, declareFlowTools } from './tools'

export function registerFlow(on: On): void {
  registerFlowTools(on)
}
