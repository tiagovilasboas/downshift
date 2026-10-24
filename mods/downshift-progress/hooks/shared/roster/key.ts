// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The key Downshift gives a harness tool call id in `session --panel`
// (`spawns[].tool_use`): sha256 of a fixed prefix and the id, as lowercase hex.

const PREFIX = 'harness-downshift-session-v1:'

/** The panel key of a tool call id; empty when Web Crypto is not there (rows draw without figures). */
export async function toolUseKey(toolUseId: string): Promise<string> {
  try {
    const bytes = new TextEncoder().encode(PREFIX + toolUseId)
    const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes)
    return Array.from(new Uint8Array(digest), b => b.toString(16).padStart(2, '0')).join('')
  } catch {
    return ''
  }
}
