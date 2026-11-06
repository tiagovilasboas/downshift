// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package testenv

import (
	"os"
	"strconv"
	"time"
)

// TimeoutScaleEnv stretches the deadlines a test gives a real child process
// (a fake CLI, a fake MCP server). Those deadlines guard against a hang, not a
// slow run, but a machine loaded by several parallel `go test` runs can take
// seconds just to start /bin/sh. The git hooks set it (Makefile HOOK_ENV); CI
// and `make check` leave it unset, so the factor is 1 there.
const TimeoutScaleEnv = "DOWNSHIFT_TEST_TIMEOUT_SCALE"

// TimeoutScale is the factor from TimeoutScaleEnv: a number of at least 1,
// and 1 when the variable is unset or not a valid number.
func TimeoutScale() float64 {
	f, err := strconv.ParseFloat(os.Getenv(TimeoutScaleEnv), 64)
	if err != nil || f < 1 {
		return 1
	}
	return f
}

// Scaled returns d stretched by TimeoutScale. Use it for a deadline a test
// expects a child process to MEET; a deadline the test expects to EXPIRE (a
// silent server must time out) stays as is, or the test only gets slower.
func Scaled(d time.Duration) time.Duration {
	return time.Duration(float64(d) * TimeoutScale())
}
