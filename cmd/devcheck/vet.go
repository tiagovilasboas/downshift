// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// vetFor returns a stage check that runs go vet ./... with env added, such as
// GOOS=windows, so code behind a build tag of another OS is type-checked too.
func vetFor(env []string) func() ([]byte, bool) {
	return func() ([]byte, bool) {
		out, err := combined(env, "go", "vet", "./...")
		return out, err == nil
	}
}
