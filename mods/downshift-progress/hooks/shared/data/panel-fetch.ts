// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Runs `downshift session --panel` through the host's process port and returns
// the parsed panel, or the reason it is not there. Never throws: every failure
// is a value the UI can draw around (fail-open).
//
// Only the slice of the engine's `$` read here is typed, so this module does not
// depend on the engine's types. Host commands exist on the CLI alone; on other
// surfaces `$.process` is absent or its call rejects.

import type { PanelFailure, PanelResult } from './panel'
import { parsePanel } from './panel-parse'

/** The wall-clock budget of one call, in milliseconds. */
export const PANEL_TIMEOUT_MS = 2000

/** The binary looked up on PATH when no override is set. */
export const DEFAULT_BINARY = 'downshift'

export type RunResult = {
  exitCode: number
  stdout: string
  stderr?: string
  isStdoutTruncated?: boolean
}

export type ProcessPort = {
  run: (argv: readonly string[], init?: { timeoutMs?: number }) => Promise<RunResult>
}

export type EnvPort = { get: (name: string) => Promise<string | undefined> }

/** The slice of the engine's `$` this module reads. */
export type HostPort = { process?: ProcessPort; env?: EnvPort }

export type FetchOptions = {
  /** False asks for the session view only (`--no-report`). */
  withReport: boolean
  /** Explicit binary path; wins over the environment override. */
  binary?: string
}

const fail = (reason: PanelFailure): PanelResult => ({ ok: false, reason })

/** Maps a rejected `$.process.run` to the reason it could not run. */
export function classifyFailure(err: unknown): PanelFailure {
  const message = err instanceof Error ? err.message : String(err)
  const code =
    typeof err === 'object' && err !== null && 'code' in err ? String(err.code) : ''
  if (code === 'ENOENT' || /ENOENT|cannot start|no such file/i.test(message)) {
    return 'not-installed'
  }
  if (/timed? ?out|timeout|still running/i.test(message)) return 'timeout'
  return 'unavailable-surface'
}

/** `DOWNSHIFT_BIN` overrides the binary; the name is a literal, as the engine requires. */
async function binaryFrom(host: HostPort): Promise<string> {
  try {
    const fromEnv = (await host.env?.get('DOWNSHIFT_BIN'))?.trim()
    return fromEnv ? fromEnv : DEFAULT_BINARY
  } catch {
    return DEFAULT_BINARY
  }
}

/**
 * Fetches the panel of one session. `sessionId` is the raw session id the host
 * reports; the binary hashes it. Missing id answers `no-session` without a call.
 */
export async function fetchPanel(
  host: HostPort,
  sessionId: string | null | undefined,
  options: FetchOptions,
): Promise<PanelResult> {
  if (!sessionId) return fail('no-session')
  try {
    const proc = host.process
    if (!proc) return fail('unavailable-surface')
    const bin = options.binary ?? (await binaryFrom(host))
    const argv = [
      bin,
      'session',
      '--panel',
      `--session=${sessionId}`,
      ...(options.withReport ? [] : ['--no-report']),
    ]
    const result = await proc.run(argv, { timeoutMs: PANEL_TIMEOUT_MS })
    if (result.exitCode !== 0) return fail('unavailable-surface')
    const panel = parsePanel(result.stdout)
    return panel ? { ok: true, panel } : fail('bad-json')
  } catch (err) {
    return fail(classifyFailure(err))
  }
}
