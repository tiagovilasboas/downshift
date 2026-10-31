// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

// gh outputs the compactor is wired to summarise on a zero exit. Each is the
// non-TTY, line-based form gh prints into a pipe.
var (
	// gh run view --log: verbose CI log, job\tstep\ttimestamp message. A
	// clean run is mostly setup noise the summary drops.
	ghRunLogClean = func() string {
		var b strings.Builder
		b.WriteString("test\tSet up job\t2026-10-30T12:00:01.1Z Current runner version: '2.320.0'\n")
		b.WriteString("test\tRun actions/checkout@v4\t2026-10-30T12:00:02.1Z Syncing repository\n")
		b.WriteString("test\tRun actions/setup-go@v5\t2026-10-30T12:00:03.1Z Setup go version spec 1.23\n")
		b.WriteString("test\tRun actions/cache@v4\t2026-10-30T12:00:04.1Z Cache restored successfully\n")
		b.WriteString("test\tgo test\t2026-10-30T12:00:05.1Z go test ./...\n")
		b.WriteString("test\tgo test\t2026-10-30T12:00:30.1Z ok  github.com/x/internal/a 0.81s\n")
		b.WriteString("test\tgo test\t2026-10-30T12:00:31.1Z ok  github.com/x/internal/b 0.43s\n")
		b.WriteString("test\tgo vet\t2026-10-30T12:00:40.1Z go vet ./...\n")
		b.WriteString("test\tPost Run actions/cache@v4\t2026-10-30T12:00:41.1Z Cache saved successfully\n")
		b.WriteString("test\tPost Run actions/setup-go@v5\t2026-10-30T12:00:42.1Z Post job cleanup\n")
		b.WriteString("test\tComplete job\t2026-10-30T12:00:44.1Z Cleaning up orphan processes\n")
		return b.String()
	}()

	// gh run list: one tab-separated run row per workflow run.
	ghRunListClean = "completed\tsuccess\tci: shard\tCI\tmain\tpush\t18234567890\t4m12s\t5m\n" +
		"completed\tsuccess\tfix: gate\tCI\tmain\tpush\t18234567810\t3m58s\t22m\n" +
		"completed\tsuccess\tdocs: matrix\tCI\tmain\tpush\t18234567700\t4m02s\t1h\n"

	// gh pr view: a header block of key:\tvalue lines, a -- separator, a body.
	ghPrViewClean = "title:\tfeat: wire gh compaction\nstate:\tOPEN\nauthor:\ttiagovilasboas\n" +
		"labels:\tenhancement\nurl:\thttps://github.com/x/y/pull/1\nadditions:\t412\n--\n" +
		"A long body an agent does not need once it has the title and state. " +
		"It describes the motivation, the design and the tests across several sentences.\n"

	// gh pr checks: one tab-separated check row, all passing.
	ghPrChecksClean = "build\tpass\t1m12s\thttps://x/1\n" +
		"test\tpass\t3m40s\thttps://x/2\n" +
		"lint\tpass\t0m48s\thttps://x/3\n"
)

// shScript wraps body as a here-doc the fake bin prints verbatim, then exits
// with code. A here-doc keeps tabs, which every gh table relies on.
func shScript(body string, code int) string {
	s := "cat <<'DS_EOF'\n" + body + "DS_EOF\n"
	if code != 0 {
		s += "exit " + itoa(code) + "\n"
	}
	return s
}

// TestRun_GhCompactionAndFailOpen is the gate-level proof for the gh-run and
// gh-pr families: a recognised shape on a zero exit compacts; everything else
// passes through byte-identical with the program's own exit code intact.
//
// 1:3 unhappy-path: one happy case (compacts) per shape, three unhappy ones
// (non-zero exit stays red and raw; an unexpected shape under the gh argv is
// never compacted; the kill switch keeps raw).
func TestRun_GhCompactionAndFailOpen(t *testing.T) {
	// A shape no gh compressor recognises: a plain sentence gh might print for
	// an empty result. It must pass through untouched under any gh argv.
	const unrecognised = "no runs found for the given filters\n"

	cases := []struct {
		name       string
		bin        string
		body       string
		code       int
		argv       []string
		compact    bool
		wantCode   int
		wantClass  string
		compacted  bool
		wantFamily Family
	}{
		// --- gh run view --log (the largest lost gain) ---
		{
			name: "gh run view --log compacts on success",
			bin:  "gh", body: ghRunLogClean, code: 0,
			argv: []string{"gh", "run", "view", "123", "--log"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, compacted: true, wantFamily: FamilyGhRun,
		},
		{
			name: "gh run view --log failure stays raw and red",
			bin:  "gh", body: ghRunLogClean, code: 1,
			argv: []string{"gh", "run", "view", "123", "--log"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, compacted: false, wantFamily: FamilyGhRun,
		},
		{
			name: "unexpected shape under gh run is not compacted",
			bin:  "gh", body: unrecognised, code: 0,
			argv: []string{"gh", "run", "view", "123", "--log"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, compacted: false, wantFamily: FamilyGhRun,
		},
		{
			name: "gh run kill switch keeps raw",
			bin:  "gh", body: ghRunLogClean, code: 0,
			argv: []string{"gh", "run", "view", "123", "--log"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, compacted: false, wantFamily: FamilyGhRun,
		},

		// --- gh run list ---
		{
			name: "gh run list compacts on success",
			bin:  "gh", body: ghRunListClean, code: 0,
			argv: []string{"gh", "run", "list"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, compacted: true, wantFamily: FamilyGhRun,
		},
		{
			name: "gh run list failure stays raw and red",
			bin:  "gh", body: ghRunListClean, code: 1,
			argv: []string{"gh", "run", "list"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, compacted: false, wantFamily: FamilyGhRun,
		},

		// --- gh pr view ---
		{
			name: "gh pr view compacts on success",
			bin:  "gh", body: ghPrViewClean, code: 0,
			argv: []string{"gh", "pr", "view", "1"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, compacted: true, wantFamily: FamilyGhPr,
		},
		{
			name: "gh pr view failure stays raw and red",
			bin:  "gh", body: ghPrViewClean, code: 1,
			argv: []string{"gh", "pr", "view", "1"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, compacted: false, wantFamily: FamilyGhPr,
		},
		{
			name: "unexpected shape under gh pr is not compacted",
			bin:  "gh", body: unrecognised, code: 0,
			argv: []string{"gh", "pr", "view", "1"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, compacted: false, wantFamily: FamilyGhPr,
		},
		{
			name: "gh pr kill switch keeps raw",
			bin:  "gh", body: ghPrViewClean, code: 0,
			argv: []string{"gh", "pr", "view", "1"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, compacted: false, wantFamily: FamilyGhPr,
		},

		// --- gh pr checks ---
		{
			name: "gh pr checks compacts on success",
			bin:  "gh", body: ghPrChecksClean, code: 0,
			argv: []string{"gh", "pr", "checks", "1"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, compacted: true, wantFamily: FamilyGhPr,
		},
		{
			name: "gh pr checks failure stays raw and red",
			bin:  "gh", body: ghPrChecksClean, code: 1,
			argv: []string{"gh", "pr", "checks", "1"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, compacted: false, wantFamily: FamilyGhPr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, shScript(tc.body, tc.code))
			r, out, errOut := run(t, tc.argv, tc.compact)
			if r.ExitCode != tc.wantCode || r.ExitClass != tc.wantClass {
				t.Fatalf("exit = %d/%s, want %d/%s (stderr %q)", r.ExitCode, r.ExitClass, tc.wantCode, tc.wantClass, errOut)
			}
			if r.Family != tc.wantFamily {
				t.Fatalf("family = %q, want %q", r.Family, tc.wantFamily)
			}
			if r.Compacted != tc.compacted {
				t.Fatalf("compacted = %v, want %v (out %q)", r.Compacted, tc.compacted, out)
			}
			if tc.compacted {
				// A compacted run is strictly shorter and carries the marker;
				// it never drops the exit code (checked above).
				if len(out) >= len(tc.body) {
					t.Fatalf("compacted output not shorter: %d >= %d", len(out), len(tc.body))
				}
				if !strings.Contains(out, "[downshift compact: compacted") {
					t.Fatalf("missing compaction marker: %q", out)
				}
			} else {
				// Not compacted: output is byte-identical to what the program
				// wrote on stdout, so no fact is ever lost.
				if out != tc.body {
					t.Fatalf("passthrough output = %q, want raw %q", out, tc.body)
				}
			}
			if errOut != "" {
				t.Fatalf("unexpected stderr: %q", errOut)
			}
		})
	}
}
