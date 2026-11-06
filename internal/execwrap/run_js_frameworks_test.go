// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_CypressPassingRunCompacts(t *testing.T) {
	const output = "  (Run Starting)\n  (Run Finished)\n  │ Tests: 2\n  │ Passing: 2\n  │ Failing: 0\n  │ Pending: 0\n  │ Skipped: 0\n  │ Duration: 1 second\n  │ Spec Ran: cypress/e2e/accounting.cy.ts\n"
	fakeBin(t, "cypress", "cat <<'EOF'\n"+output+"EOF")
	r, stdout, stderr := run(t, []string{"cypress", "run"}, true)
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero; stderr=%q", r.ExitCode, r.ExitClass, stderr)
	}
	if !r.Compacted || r.Family != FamilyCypress {
		t.Fatalf("compacted=%v family=%q, want true cypress", r.Compacted, r.Family)
	}
	if !strings.Contains(stdout, "Cypress: PASS (2 tests") || strings.Contains(stdout, "Run Starting") {
		t.Fatalf("unexpected Cypress output: %q", stdout)
	}
}

func TestRun_NextBuildAndPHPUnitCompact(t *testing.T) {
	for _, tc := range []struct {
		name, bin, command, fixture, want string
		family                            Family
	}{
		{"Next.js build", "next", "next build", "next_build_pass.txt", "Next.js Build: SUCCESS (5 routes: 3 app, 2 pages; shared First Load JS 96.7 kB in 8.2s)", FamilyNextBuild},
		{"Gradle", "gradle", "gradle build", "gradle_pass_build.txt", "gradle: 14 tasks", FamilyGradle},
		{"PHPUnit", "phpunit", "phpunit", "phpunit_pass_default.txt", "PHPUnit: PASS (58 tests, 143 assertions", FamilyPHPUnit},
		{"Flutter test", "flutter", "flutter test", "flutter_pass_compact.txt", "dart/flutter test: PASS (3 passed, 1 skipped", FamilyFlutterTest},
		{"Dart test", "dart", "dart test --no-color", "dart_pass_compact.txt", "dart/flutter test: PASS (3 passed, 1 skipped", FamilyDartTest},
		{"Maven", "mvn", "mvn test", "maven_pass_test.txt", "[INFO] BUILD SUCCESS", FamilyMaven},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+string(data)+"EOF\n")
			argv := strings.Fields(tc.command)
			r, stdout, stderr := run(t, argv, true)
			if r.ExitCode != 0 || !r.Compacted || r.Family != tc.family || stderr != "" || !strings.Contains(stdout, tc.want) {
				t.Fatalf("result=%+v stdout=%q stderr=%q", r, stdout, stderr)
			}
		})
	}
}
