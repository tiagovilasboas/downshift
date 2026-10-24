// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The two elements the pane draws with, as `$.ui.resolve(e)` returns them.

import type { EngineInterface } from 'claude-code'

export type Ui = ReturnType<EngineInterface['ui']['resolve']>
