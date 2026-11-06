// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The elements every surface draws (terminal, desktop, vscode, mobile): the
// dashboard uses only these, so it degrades to text everywhere.

import type { Elements, RenderSurface } from 'claude-code'

export type Ui = Pick<Elements[RenderSurface], 'Box' | 'Text' | 'Button'>
