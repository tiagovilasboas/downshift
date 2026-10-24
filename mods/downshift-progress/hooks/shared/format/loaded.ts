// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

/** The one line the status command answers. */
export function loadedLine(name: string, version: string): string {
  return `${name} ${version} loaded`
}
