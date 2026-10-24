// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

/** Every label the band and the agents pane draw. Scope labels come from the binary as is. */
export const MESSAGE_KEYS = [
  'running',
  'finished',
  'failed',
  'planned',
  'tokensSaved',
  'tokens',
  'time',
  'held',
  'model',
  'effort',
  'round',
  'subagents',
  'noSubagents',
  'downshiftNotFound',
] as const

export type MessageKey = (typeof MESSAGE_KEYS)[number]
export type Messages = Readonly<Record<MessageKey, string>>
