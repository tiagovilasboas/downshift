// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package archtest enforces the import graph documented in
// docs/design/architecture.md (and AGENTS.md "Key invariants"). It has no
// production code: the test asks `go list -deps` for the transitive production
// imports of a package and fails when one is forbidden. Test files are not part
// of that graph, which is the point: a core test may import nbtier, the binary
// does not.
package archtest

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

const module = "github.com/tiagovilasboas/downshift/internal/"

// rule: the production dependency closure of pkg must not contain any of
// forbidden (each matches the package itself and its sub-packages).
type rule struct {
	pkg       string
	forbidden []string
	why       string
}

var rules = []rule{
	{"core", []string{"quota", "nbtier", "nbopinion", "catalog", "telemetry", "adapt", "policyfile", "scoreindex", "adapters"},
		"core production sources reach those packages only through hooks registered by cmd"},
	{"session", []string{"core"}, "session is a leaf loader; PlanForSession and friends stay in core"},
	{"fuse", []string{"core"}, "fuse imports classify and semantic, never core (core wraps fuse)"},
	{"codeintel", []string{"core", "classify"}, "code intelligence is independent of routing"},
	{"replay", []string{"core", "catalog", "telemetry", "adapters", "session", "classify", "hookport"}, "the replay domain reaches the router, the catalog and the transcripts only through its ports (Source, Router, Comparer)"},
}

// violations returns the entries of deps (import paths) that match a forbidden
// package. Pure, so the negative test can prove it detects a violation.
func violations(deps, forbidden []string) []string {
	var out []string
	for _, d := range deps {
		if !strings.HasPrefix(d, module) {
			continue
		}
		rel := strings.TrimPrefix(d, module)
		for _, f := range forbidden {
			if rel == f || strings.HasPrefix(rel, f+"/") {
				out = append(out, rel)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func TestImportGraph(t *testing.T) {
	for _, r := range rules {
		t.Run(r.pkg, func(t *testing.T) {
			// -deps without -test lists production imports only.
			out, err := exec.Command("go", "list", "-deps", module+r.pkg).Output()
			if err != nil {
				t.Fatalf("go list -deps %s: %v", r.pkg, err)
			}
			deps := strings.Fields(string(out))
			if len(deps) == 0 {
				t.Fatalf("no dependencies listed for %s", r.pkg)
			}
			if v := violations(deps, r.forbidden); len(v) > 0 {
				t.Errorf("internal/%s must not depend on %v (%s)", r.pkg, v, r.why)
			}
		})
	}
}

func TestViolationsDetectsForbiddenImports(t *testing.T) {
	deps := []string{
		"fmt",
		module + "core",
		module + "classify",
		module + "quotamodel", // allowed: pure types, not quota
		module + "telemetry",
		module + "adapters/claudecode",
		"github.com/other/telemetry", // outside the module: ignored
	}
	got := violations(deps, []string{"quota", "telemetry", "adapters"})
	want := []string{"adapters/claudecode", "telemetry"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("violations = %v, want %v", got, want)
	}
	if v := violations(deps, []string{"nbtier"}); len(v) != 0 {
		t.Fatalf("false positive: %v", v)
	}
}
