// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The one place the mod adds numbers: the four usage fields of a turn into
// "tokens this agent used". That is an observation of this session, not Downshift
// accounting; every saved-token figure is printed as the binary wrote it.

export type UsageLike = {
  input_tokens: number
  output_tokens: number
  cache_read_input_tokens: number
  cache_creation_input_tokens: number
}

/** Tokens one turn used; a missing usage is 0. */
export function usageTotal(usage: UsageLike | null | undefined): number {
  if (!usage) return 0
  return usage.input_tokens + usage.output_tokens + usage.cache_read_input_tokens + usage.cache_creation_input_tokens
}

/** `842`, `12.3k`, `1.2M`: a token count of this agent in the CLI's short style. */
export function formatTokens(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0'
  if (n < 1000) return String(Math.round(n))
  const [value, unit] = n < 1_000_000 ? [n / 1000, 'k'] : [n / 1_000_000, 'M']
  return `${value >= 100 ? Math.round(value) : Math.round(value * 10) / 10}${unit}`
}
