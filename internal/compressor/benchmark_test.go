// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func BenchmarkCompress_GoTest(b *testing.B) {
	in := []byte("ok  github.com/foo/bar  0.1s  coverage: 80%\nok  github.com/foo/baz  0.2s  coverage: 90%\nok  github.com/foo/qux  0.3s  coverage: 85%\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compressor.Compress(in, compressor.ModeSafe)
	}
}

func BenchmarkCompress_GitStatus(b *testing.B) {
	in := []byte("On branch main\nYour branch is up to date.\nnothing to commit, working tree clean\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compressor.Compress(in, compressor.ModeSafe)
	}
}

func BenchmarkCompress_RepetitiveLogs(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("2026-10-08 17:00:00 INFO request processed\n")
	}
	in := []byte(sb.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compressor.Compress(in, compressor.ModeSafe)
	}
}
