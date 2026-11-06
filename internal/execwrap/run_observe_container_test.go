// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/testenv"
)

func TestRun_WaitsForObserveHookUnderDeadline(t *testing.T) {
	fakeBin(t, "go", "printf 'not a go test output\\n'")
	prevHook, prevDeadline := ObserveUncompactedHook, ObserveHookDeadline
	defer func() { ObserveUncompactedHook, ObserveHookDeadline = prevHook, prevDeadline }()

	var done atomic.Bool
	ObserveUncompactedHook = func(argv []string, sample []byte, family Family) {
		time.Sleep(50 * time.Millisecond)
		if string(sample) == "not a go test output\n" && family == FamilyGoTest {
			done.Store(true)
		}
	}
	r, out, _ := run(t, []string{"go", "test"}, true)
	if r.ExitCode != 0 || r.Compacted || out != "not a go test output\n" {
		t.Fatalf("exit=%d compacted=%v out=%q", r.ExitCode, r.Compacted, out)
	}
	if !done.Load() {
		t.Error("Run returned before the hook finished")
	}

	// A hook that overruns the deadline neither blocks Run nor changes the exit code.
	ObserveHookDeadline = 20 * time.Millisecond
	ObserveUncompactedHook = func([]string, []byte, Family) { time.Sleep(2 * time.Second) }
	start := time.Now()
	r, _, _ = run(t, []string{"go", "test"}, true)
	// The hook sleeps 2 s: finishing under 2 s proves Run did not wait for it
	// (always checked); the tighter 1 s bound is a performance budget.
	elapsed := time.Since(start)
	if r.ExitCode != 0 || elapsed >= 2*time.Second {
		t.Fatalf("overrunning hook changed the result: exit=%d elapsed=%s", r.ExitCode, elapsed)
	}
	testenv.CheckBudget(t, "Run with an overrunning hook", elapsed, time.Second)

	// A panicking hook is swallowed.
	ObserveUncompactedHook = func([]string, []byte, Family) { panic("boom") }
	if r, _, _ = run(t, []string{"go", "test"}, true); r.ExitCode != 0 {
		t.Fatalf("panicking hook changed the exit code: %d", r.ExitCode)
	}
}

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
