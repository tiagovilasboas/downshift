// Downshift OpenCode bridge module (wire v1). Written by `downshift setup`; edit nothing here.
//
// The calls to the `downshift` binary: one JSON answer per call, fail-open.

import { spawnSync } from "node:child_process"
import { createHash } from "node:crypto"

export const HARNESS = "opencode"
export const WIRE_VERSION = 1
const CALL_TIMEOUT_MS = 3000
// A shell call runs synchronously inside OpenCode's create.before hook: the
// handler mutates event.command and returns, so the hook cannot be made async
// and the event loop waits for it. The Go side reads stdin for at most 2 s, so
// waiting longer than that buys nothing; the shell call uses the same bound.
export const SHELL_TIMEOUT_MS = 2000
const MAX_OUTPUT_BYTES = 1 << 20
// OpenCode prefixes a child's first prompt with this line before the task the
// parent gave (checked against 2.2.5). Removing it keeps the classification
// input to the task itself; a prompt without it is forwarded whole.
const SUBAGENT_PREFIX = "You are a subagent spawned by another session.\n"

// opaque is the correlation id the wire carries instead of a session id: a
// stable, lowercase-hex digest, which is what the adapter validates.
export function opaque(value: string): string {
  return createHash("sha256").update(`opencode\0${value}`).digest("hex")
}

export function count(value: any): number {
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0
}

export function taskText(text: string): string {
  return text.startsWith(SUBAGENT_PREFIX) ? text.slice(SUBAGENT_PREFIX.length) : text
}

// answer runs the binary once and reads the single JSON line it answers.
// Anything else is null, and every caller treats null as "leave it alone".
export function answer(bin: string, args: string[], payload: unknown, timeout: number = CALL_TIMEOUT_MS): any {
  try {
    const result = spawnSync(bin, args, {
      input: JSON.stringify(payload),
      encoding: "utf8",
      timeout,
      maxBuffer: MAX_OUTPUT_BYTES,
      windowsHide: true,
    })
    if (!result || result.error || result.status !== 0) return null
    if (typeof result.stdout !== "string") return null
    const line = result.stdout
      .split("\n")
      .map((s) => s.trim())
      .find((s) => s.length > 0)
    if (!line || line.length > MAX_OUTPUT_BYTES) return null
    const parsed = JSON.parse(line)
    return parsed && typeof parsed === "object" ? parsed : null
  } catch {
    return null
  }
}

// batchAnswer sends one observation batch to an observer command.
export function batchAnswer(bin: string, command: string, batch: string): void {
  try {
    spawnSync(bin, [command, HARNESS], {
      input: batch,
      encoding: "utf8",
      timeout: CALL_TIMEOUT_MS,
      maxBuffer: MAX_OUTPUT_BYTES,
      windowsHide: true,
    })
  } catch {
    // Observers never block a finished child.
  }
}
