// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
export type Available = { files: boolean; agents: boolean }
export type DciMode = 'active' | 'observe' | 'off'
export type Status = { dci: DciMode; compactor: boolean }
export type Mode = 'economy' | 'performance'

declare module 'claude-code' {
  interface PluginState {
    'downshift-dock': { available: Available | null; status: Status | null; mode: Mode | null }
  }
}
