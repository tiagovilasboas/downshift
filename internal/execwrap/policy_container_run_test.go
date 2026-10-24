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

// fakeBin writes a POSIX shell script, so this run test is Unix-only like
// run_test.go; the policy table tests next to it build everywhere.
// TestRun_ContainerExecKeepsExitCode runs a fake docker that prints a PHPUnit
// transcript. A zero exit compacts under the inner family; a non-zero exit
// keeps the raw output and the real exit code.
func TestRun_ContainerExecKeepsExitCode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", "phpunit_pass_default.txt"))
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Fields("docker compose exec -T app vendor/bin/phpunit")

	fakeBin(t, "docker", "cat <<'EOF'\n"+string(data)+"EOF\nexit 0")
	r, stdout, stderr := run(t, argv, true)
	if r.ExitCode != 0 || !r.Compacted || r.Family != FamilyPHPUnit || stderr != "" || !strings.Contains(stdout, "PHPUnit: PASS (58 tests, 143 assertions") {
		t.Fatalf("pass: result=%+v stdout=%q stderr=%q", r, stdout, stderr)
	}

	fakeBin(t, "docker", "cat <<'EOF'\n"+string(data)+"EOF\nexit 3")
	r, stdout, _ = run(t, argv, true)
	if r.ExitCode != 3 || r.Compacted || r.Family != FamilyPHPUnit || stdout != string(data) {
		t.Fatalf("fail: result=%+v stdout=%q", r, stdout)
	}
}
