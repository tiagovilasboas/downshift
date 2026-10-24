// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The agents pane: header tile, then Running / Finished / Planned, or the empty
// state. No Button anywhere (owner decision): the pane opens and closes by
// `/downshift-agents`, and finished rows leave by themselves.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { On } from 'claude-code'
import { resolveLang } from '../../shared/i18n/lang'
import { t } from '../../shared/i18n/t'
import { prefersAsciiGlyphs } from '../../shared/format/glyph'
import { readRoster } from '../../shared/roster'
import { paletteFor } from '../../shared/theme/tokens'
import { PANE_ID } from './command'
import { headerTile, isEmpty, sections } from './Sections'

const COLUMNS = 80

export function registerPane(on: On): void {
  on('ui.render', { component: 'Pane', requestId: PANE_ID }, async ($, e) => {
    const ui = $.ui.resolve(e)
    const read = async <T,>(get: () => Promise<T>): Promise<T | undefined> => {
      try {
        return await get()
      } catch {
        return undefined
      }
    }
    const setting = await read(() => $.settings.read({ key: 'language' }))
    const theme = await read(() => $.settings.read({ key: 'theme' }))
    const lang = resolveLang([typeof setting === 'string' ? setting : undefined, await read(() => $.env.get('LANG'))])
    const term = await read(() => $.env.get('TERM'))
    const nowMs = await $.clock.now()
    const roster = await readRoster($, nowMs)
    const env = {
      ui,
      lang,
      nowMs,
      palette: paletteFor(typeof theme === 'string' ? theme : undefined),
      columns: e.props.bodyColumns ?? COLUMNS,
      isAscii: prefersAsciiGlyphs(term, undefined),
    }
    const { Box, Text } = ui
    return (
      <Box flexDirection="column" gap={1}>
        {headerTile(roster.report, env)}
        {isEmpty(roster) ? <Text color={env.palette.muted}>{t('noSubagents', lang)}</Text> : sections(roster, env)}
      </Box>
    )
  })
}
