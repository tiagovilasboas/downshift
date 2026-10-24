// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

/**
 * Cut `title` to at most `width` characters (code points, not UTF-16 units),
 * ending with `ellipsis` when it had to be cut. A width too small for the
 * ellipsis keeps only its first characters.
 */
export function truncateTitle(title: string, width: number, ellipsis = '…'): string {
  const limit = Math.floor(width)
  if (!(limit > 0)) return ''
  const chars = Array.from(title)
  if (chars.length <= limit) return title
  const mark = Array.from(ellipsis)
  if (mark.length >= limit) return mark.slice(0, limit).join('')
  return chars.slice(0, limit - mark.length).join('').trimEnd() + ellipsis
}
