// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// MaxLineBytes is the longest event-log line the readers accept. A line
// above it is reported as an error instead of silently truncating the log.
const MaxLineBytes = 1 << 20
