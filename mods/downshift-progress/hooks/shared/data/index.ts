// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Data access to `downshift session --panel`. Downshift by Tiago Vilas Boas ·
// https://github.com/tiagovilasboas/downshift

export type { Panel, PanelFailure, PanelResult, PanelReport, PanelSpawn } from './panel'
export { parsePanel } from './panel-parse'
export {
  DEFAULT_BINARY,
  PANEL_TIMEOUT_MS,
  classifyFailure,
  fetchPanel,
} from './panel-fetch'
export type { EnvPort, FetchOptions, HostPort, ProcessPort, RunResult } from './panel-fetch'
export { NO_REPORT, REPORT_MAX_AGE_MS, rememberReport, reportOlderThan } from './report-cache'
export type { ReportCache } from './report-cache'
