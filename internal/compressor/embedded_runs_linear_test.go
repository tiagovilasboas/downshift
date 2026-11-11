// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A refused block is skipped whole, so a large log with many openers inside one
// refused block folds in linear time. Before, each opener rescanned to the same
// end: a 637 KB go test -v log with logged output took over a minute.
func TestFoldEmbeddedRuns_RefusedBlockStaysLinear(t *testing.T) {
	cases := map[string]func(i int) string{
		"go -v with logged output": func(i int) string {
			return fmt.Sprintf("=== RUN   TestX%d\n    x_test.go:10: log line %d\n--- PASS: TestX%d (0.00s)\n", i, i, i)
		},
		"go -v with no ok line": func(i int) string {
			return fmt.Sprintf("=== RUN   TestX%d\n--- PASS: TestX%d (0.00s)\n", i, i)
		},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			var b strings.Builder
			b.WriteString("make test\n")
			for i := 0; i < 8000; i++ {
				b.WriteString(row(i))
			}
			if strings.Contains(name, "logged") {
				b.WriteString("PASS\nok  \texample.com/x\t0.5s\n")
			}
			in := []byte(b.String())
			start := time.Now()
			r := FoldEmbeddedRuns(in)
			if took := time.Since(start); took > 2*time.Second {
				t.Fatalf("fold of %d bytes took %v; want linear time", len(in), took)
			}
			if r.Applied {
				t.Fatalf("refused block folded: %s", r.Reason)
			}
		})
	}
}
