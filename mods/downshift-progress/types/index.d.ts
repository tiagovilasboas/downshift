// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// State contract of the downshift-progress mod. Type-check with the tsconfig.json
// beside this folder (`tsc -p mods/downshift-progress`). Values are added by the
// feature tasks (roster, flow, downshift data); empty on purpose in the scaffold.

declare module 'claude-code' {
  interface PluginState {
    'downshift-progress': {}
  }
}
