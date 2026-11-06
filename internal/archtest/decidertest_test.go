// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDeciderStubsAreTestOnly: the fixed deciders in internal/core/decidertest
// exist to pin contract tests. No production file may import them, or a stub
// could end up routing real spawns. -f .Imports excludes test files.
func TestDeciderStubsAreTestOnly(t *testing.T) {
	const stub = module + "core/decidertest"
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list ./...: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		for _, imp := range fields[1:] {
			if imp == stub {
				t.Errorf("%s imports %s in production code; stubs are test-only", fields[0], stub)
			}
		}
	}
}
