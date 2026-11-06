// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The tool each kind of agent holds in its left hand, drawn at cols 0-2 beside
// the body. The kind comes from the agent type the engine reports; a type that
// names no known role leaves the hands free.

import { stamp } from './pixels'
import type { Grid } from './pixels'

type Tool = { name: string; top: number; art: readonly string[] }

export const TOOLS = {
  coder: { name: 'laptop', top: 4, art: ['GGG', 'GBG', 'GBG', 'MMM'] },
  'coder-back': { name: 'server rack', top: 3, art: ['HHH', 'HVH', 'HHH', 'HVH', 'HHH'] },
  'coder-front': { name: 'browser window', top: 4, art: ['BBB', 'CCC', 'CVC', 'CCC'] },
  refactor: { name: 'broom', top: 3, art: ['.Y.', '.Y.', '.Y.', '.Y.', 'YOY', 'OOO'] },
  reviewer: { name: 'clipboard', top: 4, art: ['.G.', 'YCY', 'YGY', 'YCY', 'YYY'] },
  tester: { name: 'flask', top: 4, art: ['.G.', '.G.', 'GVG', 'VVV'] },
  debugger: { name: 'bug', top: 4, art: ['G.G', '.H.', 'HHH', 'G.G'] },
  security: { name: 'shield', top: 4, art: ['BBB', 'BCB', 'BBB', '.B.'] },
  researcher: { name: 'magnifier', top: 3, art: ['GGG', 'GBG', 'GGG', '.G.', '.G.'] },
  documenter: { name: 'pencil', top: 2, art: ['.H.', '.Y.', '.Y.', '.Y.', '.Y.', '.M.'] },
  architect: { name: 'set square', top: 4, art: ['G..', 'GG.', 'G.G', 'GGG'] },
  platform: { name: 'wrench', top: 3, art: ['G.G', 'GGG', '.G.', '.G.', '.G.'] },
  ai: { name: 'spark', top: 2, art: ['O.O', '.O.', 'O.O', '.G.', '.G.'] },
  performance: { name: 'stopwatch', top: 3, art: ['.G.', 'GGG', 'GCG', 'GGG'] },
  mechanical: { name: 'hammer', top: 3, art: ['HHH', 'HHH', '.Y.', '.Y.', '.Y.'] },
} as const satisfies Record<string, Tool>

export type Kind = keyof typeof TOOLS

/** Words of an agent type that name each kind, checked in this order (the first match wins). */
const WORDS: readonly [Kind, readonly string[]][] = [
  ['mechanical', ['mechanical']],
  ['refactor', ['refactor']],
  ['reviewer', ['reviewer', 'review']],
  ['debugger', ['debugger', 'debug']],
  ['tester', ['tester', 'test', 'qa']],
  ['security', ['security', 'appsec']],
  ['researcher', ['researcher', 'research', 'explore']],
  ['documenter', ['documenter', 'writer', 'docs']],
  ['architect', ['architect', 'plan']],
  ['performance', ['performance', 'perf']],
  ['platform', ['platform', 'devops', 'backend', 'infra']],
  ['ai', ['ai', 'finops']],
  ['coder-back', ['back']],
  ['coder-front', ['front', 'frontend']],
  ['coder', ['coder', 'implementer', 'fullstack']],
]

/** The kind an agent type names, by whole words, or undefined when it names none. */
export function kindOf(type: string | undefined): Kind | undefined {
  const words = new Set((type ?? '').toLowerCase().split(/[^a-z0-9]+/))
  return WORDS.find(([, names]) => names.some(n => words.has(n)))?.[0]
}

/** The tool of `kind` in the left hand, raised `dy` cells (negative is up). */
export function holdTool(g: Grid, kind: Kind | undefined, dy = 0): Grid {
  if (kind === undefined) return g
  const t = TOOLS[kind]
  return stamp(g, t.art, 0, t.top + dy)
}
