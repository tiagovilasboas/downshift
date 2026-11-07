// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

// SpawnedFromSubagent reports a spawn issued from inside a subagent (a nested
// spawn). The payload's session_id and transcript_path are still the main
// session's, so the shared policy decides what that allows
// (core.HarnessCapabilities.TranscriptNamesSpawner).
func (ev Event) SpawnedFromSubagent() bool { return ev.AgentID != "" }
