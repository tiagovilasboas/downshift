// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

// Routing v2 (docs/design/routing-v2.md section 7) lives under
// internal/routing/. Every package there may import directly only the
// standard library, internal/core, internal/paths and other routing
// packages; its production closure never reaches the packages below. The
// rule applies to every routing package, including ones added later.
var (
	routingDirectAllowed = []string{"core", "paths"}
	routingForbidden     = []string{"telemetry", "accounting", "quota", "catalog", "adapters",
		"execwrap", "compressor", "codeintel"}
)

// routingPackages lists internal/routing/... as module-relative paths.
func routingPackages(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", module+"routing/...").Output()
	if err != nil {
		t.Fatalf("go list routing/...: %v", err)
	}
	var pkgs []string
	for _, p := range strings.Fields(string(out)) {
		pkgs = append(pkgs, strings.TrimPrefix(p, module))
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages under internal/routing")
	}
	return pkgs
}

// outsideRoutingAllowlist drops sibling routing packages, then applies the
// allowlist. Pure, so the negative test can prove it detects a violation.
func outsideRoutingAllowlist(imports []string) []string {
	var rest []string
	for _, i := range imports {
		if i != "routing" && !strings.HasPrefix(i, "routing/") {
			rest = append(rest, i)
		}
	}
	return outsideAllowlist(rest, routingDirectAllowed)
}

func TestRoutingImports(t *testing.T) {
	for _, pkg := range routingPackages(t) {
		t.Run(pkg, func(t *testing.T) {
			if bad := outsideRoutingAllowlist(directImports(t, pkg)); len(bad) > 0 {
				t.Errorf("internal/%s imports %v; routing may import only %v and routing/*", pkg, bad, routingDirectAllowed)
			}
			out, err := exec.Command("go", "list", "-deps", module+pkg).Output()
			if err != nil {
				t.Fatalf("go list -deps %s: %v", pkg, err)
			}
			if v := violations(strings.Fields(string(out)), routingForbidden); len(v) > 0 {
				t.Errorf("internal/%s must not depend on %v (routing is pure policy; I/O and surfaces plug in from outside)", pkg, v)
			}
		})
	}
}

func TestRoutingAllowlistDetectsViolations(t *testing.T) {
	got := outsideRoutingAllowlist([]string{"core", "paths", "routing/contract", "routing", "telemetry", "quotamodel"})
	if strings.Join(got, ",") != "telemetry,quotamodel" {
		t.Fatalf("outsideRoutingAllowlist = %v, want [telemetry quotamodel]", got)
	}
	deps := []string{module + "core", module + "quotamodel", module + "quota", module + "adapters/kirocrew"}
	if v := violations(deps, routingForbidden); strings.Join(v, ",") != "adapters/kirocrew,quota" {
		t.Fatalf("violations = %v, want [adapters/kirocrew quota]", v)
	}
}
