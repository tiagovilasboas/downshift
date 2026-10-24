// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Draws band lines. Text only: the band has no Button, so nothing here takes a press.

import type { Color } from 'claude-code'

import type { Token } from '../../shared/theme/tokens'
import type { Line } from './line'

/** Each brand token follows the person's theme through the closest theme key. */
export const THEME_KEY: Readonly<Record<Token, Color>> = {
  text: 'text',
  muted: 'inactive',
  accent: 'success',
  success: 'success',
  warning: 'warning',
  danger: 'error',
}

type Ui = { Box: any; Text: any }

export function bandTree({ Box, Text }: Ui, lines: readonly Line[]) {
  return (
    <Box flexDirection="column">
      {lines.map((line, row) => (
        <Text key={`line-${row}`} wrap="truncate-end">
          {line.map((segment, i) => (
            <Text key={`seg-${i}`} color={THEME_KEY[segment.token]} bold={segment.bold === true}>
              {segment.text}
            </Text>
          ))}
        </Text>
      ))}
    </Box>
  )
}
