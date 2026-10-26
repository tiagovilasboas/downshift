// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"path/filepath"
	"testing"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func present(paths ...string) func(string) bool {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

func TestResolveKiroCLI_OverrideWinsWhenPresent(t *testing.T) {
	ov := filepath.Join(t.TempDir(), "kiro-cli")
	got := resolveKiroCLI(envOf(map[string]string{EnvKiroBin: ov}), "/bundle/kiro-cli-chat", present(ov, "/bundle/kiro-cli-chat"))
	if got != ov {
		t.Fatalf("override must win, got %q", got)
	}
}

func TestResolveKiroCLI_RelativeOrMissingOverrideIsIgnored(t *testing.T) {
	got := resolveKiroCLI(envOf(map[string]string{EnvKiroBin: "kiro-cli"}), "/bundle/kiro-cli-chat", present("/bundle/kiro-cli-chat"))
	if got != "/bundle/kiro-cli-chat" {
		t.Fatalf("a relative override is refused, got %q", got)
	}
	got = resolveKiroCLI(envOf(map[string]string{EnvKiroBin: "/gone/kiro-cli"}), "/bundle/kiro-cli-chat", present("/bundle/kiro-cli-chat"))
	if got != "/bundle/kiro-cli-chat" {
		t.Fatalf("a missing override falls through, got %q", got)
	}
}

func TestResolveKiroCLI_BundledBeforePath(t *testing.T) {
	got := resolveKiroCLI(envOf(nil), "/bundle/kiro-cli-chat", present("/bundle/kiro-cli-chat"))
	if got != "/bundle/kiro-cli-chat" {
		t.Fatalf("the bundled copy must win over PATH, got %q", got)
	}
}

func TestResolveKiroCLI_NothingFoundUsesPathName(t *testing.T) {
	got := resolveKiroCLI(envOf(nil), "", present())
	if got != "kiro-cli" {
		t.Fatalf("with nothing found, fall back to the PATH name, got %q", got)
	}
}
