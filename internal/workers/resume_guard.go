// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "errors"

// errBadSessionID refuses a resume whose id fails validSessionID (cleanup.go).
// The id sits as a positional argument after `codex exec resume`, so one that
// started with "-" would read as a flag; the pattern never allows that.
var errBadSessionID = errors.New("the worker's session id has an unexpected shape; start a new worker instead")
