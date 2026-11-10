// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

func TestWithHarness(t *testing.T) {
	cases := []struct{ in, harness, want string }{
		{"'/bin/ds' compact -- go test", "kiro", "'/bin/ds' compact --harness=kiro -- go test"},
		{"'/bin/ds' compact --session=ab12 -- git status", "codex", "'/bin/ds' compact --harness=codex --session=ab12 -- git status"},
		{"'/bin/ds' compact -- a && '/bin/ds' compact -- b", "cursor", "'/bin/ds' compact --harness=cursor -- a && '/bin/ds' compact --harness=cursor -- b"},
		{"'/bin/ds' compact --harness=kiro -- x", "codex", "'/bin/ds' compact --harness=kiro -- x"},
		{"'/bin/ds' compact -- x", "Bad Name", "'/bin/ds' compact -- x"},
		{"'/bin/ds' compact -- x", "", "'/bin/ds' compact -- x"},
	}
	for _, c := range cases {
		if got := WithHarness(c.in, c.harness); got != c.want {
			t.Errorf("WithHarness(%q, %q) = %q, want %q", c.in, c.harness, got, c.want)
		}
	}
}
