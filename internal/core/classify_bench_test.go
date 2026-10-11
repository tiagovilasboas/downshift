// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// BenchmarkClassifyTypical measures a warm classification of a typical spawn
// prompt (a few hundred bytes). Run: go test ./internal/core -bench ClassifyTypical -run xxx
func BenchmarkClassifyTypical(b *testing.B) {
	prompt := "Refactor the token validation in auth/session.go to return typed errors, update the callers in api/handlers, and extend the table-driven tests."
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ClassifyWithSemantic(prompt)
	}
}
