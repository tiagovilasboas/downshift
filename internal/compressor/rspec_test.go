// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

const rspecGreenTail = "\nFinished in 3.13 seconds (files took 1.21 seconds to load)\n74 examples, 0 failures\n"

func TestCompressRspecKeepsIndentedNoticesRaw(t *testing.T) {
	for name, in := range map[string]string{
		"indented warning":     "..........\n  warning: unused variable foo\n" + rspecGreenTail,
		"indented Deprecation": "..........\n  Deprecation: should syntax is old\n" + rspecGreenTail,
		"DEPRECATION banner":   "DEPRECATION WARNING: Using `should` is deprecated\n..........\n" + rspecGreenTail,
		"notice in slowest": "..........\n\nTop 1 slowest example (0.5 seconds, 10% of total time):\n" +
			"  Foo works\n    0.5 seconds ./spec/foo_spec.rb:1\n  Deprecation: old api\n" + rspecGreenTail,
	} {
		t.Run(name, func(t *testing.T) {
			out, applied, _ := compressRspec([]byte(in))
			if applied || string(out) != in {
				t.Fatalf("must stay raw: applied=%v out=%q", applied, out)
			}
		})
	}
}

func TestCompressRspecSummarisesSlowestBlock(t *testing.T) {
	in := "..........\n\nTop 1 slowest example (0.5 seconds, 10% of total time):\n  Foo works\n    0.5 seconds ./spec/foo_spec.rb:1\n" + rspecGreenTail
	out, applied, _ := compressRspec([]byte(in))
	if !applied || !strings.HasPrefix(string(out), "PASS rspec: 74 examples, 0 failures") {
		t.Fatalf("green run with slowest block must fold: applied=%v out=%q", applied, out)
	}
}
