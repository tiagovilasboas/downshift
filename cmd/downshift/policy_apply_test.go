// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// profileJSON renders one routing.profiles entry for claude-code. An empty
// description leaves the key out.
func profileJSON(name, tier, description string) string {
	m := map[string]string{"name": name, "tier": tier, "harness": "claude-code"}
	if description != "" {
		m["description"] = description
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// routingPolicy wraps profile entries in the canonical policy shape.
func routingPolicy(profiles ...string) string {
	return `{"version":1,"routing":{"profiles":[` + strings.Join(profiles, ",") + `]}}`
}

// setupPolicyApplyEnv writes the project policy under a fresh project dir and
// returns it with an empty HOME.
func setupPolicyApplyEnv(t *testing.T, policyJSON string) (dir, home string) {
	t.Helper()
	dir = t.TempDir()
	home = t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".downshift", "policy.json"), policyJSON, 0o644)
	return dir, home
}

func writeTestFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil { // WriteFile applies the umask
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func missing(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

// runApply runs policy apply for claude-code with --yes unless mod changes it.
func runApply(t *testing.T, dir, home string, mod ...func(*policyApplyOptions)) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	opts := policyApplyOptions{Harness: "claude-code", Yes: true, Dir: dir, Home: home, Out: &out, Err: &errOut, Now: time.Now}
	for _, m := range mod {
		m(&opts)
	}
	return executePolicyApply(opts), out.String(), errOut.String()
}

func tierModel(tier core.Tier) string {
	return catalog.Load().ModelFor("claude-code", tier).ID
}

func TestPolicyApply_CompilationCreatesNewAgent(t *testing.T) {
	dir, home := setupPolicyApplyEnv(t, routingPolicy(
		profileJSON("researcher", "small", "Reads code and reports"),
		profileJSON("coder", "mid", "Writes the change"),
	))
	if code, _, stderr := runApply(t, dir, home); code != 0 {
		t.Fatalf("exit %d; stderr: %s", code, stderr)
	}
	for name, tier := range map[string]core.Tier{"researcher": core.TierSmall, "coder": core.TierMid} {
		body := readTestFile(t, filepath.Join(dir, ".claude", "agents", name+".md"))
		for _, want := range []string{"name: " + name + "\n", "model: " + tierModel(tier) + "\n", "description: "} {
			if !strings.Contains(body, want) {
				t.Errorf("%s.md missing %q:\n%s", name, want, body)
			}
		}
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(dir, ".claude", "agents", "coder.md"))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o644 {
			t.Errorf("new agent mode = %v, want 0644", fi.Mode().Perm())
		}
	}
}

func TestPolicyApply_DryRunDoesNotTouchFiles(t *testing.T) {
	dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("scout", "small", "Scouts")))
	code, stdout, stderr := runApply(t, dir, home, func(o *policyApplyOptions) { o.DryRun, o.Yes = true, false })
	if code != 0 {
		t.Fatalf("dry run exit %d: %s", code, stderr)
	}
	if !missing(t, filepath.Join(dir, ".claude")) {
		t.Error("files created during dry run")
	}
	if !strings.Contains(stdout, "Dry run: nothing was written") || !strings.Contains(stdout, "+description:") {
		t.Errorf("dry run output lacks the notice or the diff:\n%s", stdout)
	}
}

func TestPolicyApply_UpdatesExistingAgentPreservingBodyAndKeys(t *testing.T) {
	dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("tester", "small", "")))
	existing := "---\nname: tester\nmodel: claude-opus-old\ntools: [Bash, Read]\n---\n\n# Test instructions\nExecute unit tests thoroughly.\n"
	path := filepath.Join(dir, ".claude", "agents", "tester.md")
	writeTestFile(t, path, existing, 0o644)

	if code, _, stderr := runApply(t, dir, home); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := readTestFile(t, path)
	want := strings.Replace(existing, "claude-opus-old", tierModel(core.TierSmall), 1)
	if got != want {
		t.Errorf("only the model line may change:\n got: %q\nwant: %q", got, want)
	}
}

func TestPolicyApply_Idempotent(t *testing.T) {
	dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("worker", "small", "Works")))
	if code, _, _ := runApply(t, dir, home); code != 0 {
		t.Fatalf("first apply exit %d", code)
	}
	code, stdout, _ := runApply(t, dir, home)
	if code != 0 || !strings.Contains(stdout, "already up to date") {
		t.Errorf("second apply exit %d, not reported as up to date:\n%s", code, stdout)
	}
}

func TestPolicyApply_CreatesBackupBeforeModification(t *testing.T) {
	dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("writer", "mid", "")))
	agentDir := filepath.Join(dir, ".claude", "agents")
	orig := "---\nname: writer\nmodel: old\n---\nBody\n"
	writeTestFile(t, filepath.Join(agentDir, "writer.md"), orig, 0o644)

	if code, _, stderr := runApply(t, dir, home); code != 0 {
		t.Fatalf("apply exit %d: %s", code, stderr)
	}
	matches, _ := filepath.Glob(filepath.Join(agentDir, "writer.md.downshift-bak-*"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 backup, found %d", len(matches))
	}
	if got := readTestFile(t, matches[0]); got != orig {
		t.Errorf("backup content mismatch: got %q, want %q", got, orig)
	}
}

func TestPolicyApply_IgnoresProfilesOfOtherHarnesses(t *testing.T) {
	other := `{"name":"elsewhere","tier":"small","harness":"codex","description":"Not for Claude Code"}`
	dir, home := setupPolicyApplyEnv(t, routingPolicy(other, profileJSON("here", "small", "For Claude Code")))
	if code, _, stderr := runApply(t, dir, home); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !missing(t, filepath.Join(dir, ".claude", "agents", "elsewhere.md")) {
		t.Error("a codex profile was written as a Claude Code agent")
	}
	if missing(t, filepath.Join(dir, ".claude", "agents", "here.md")) {
		t.Error("the claude-code profile was not written")
	}
}

func TestPolicyApply_CLIValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runPolicyApply([]string{}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("missing harness exit %d, want 2", code)
	}
	stderr.Reset()
	if code := runPolicyApply([]string{"--harness", "cursor"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("unsupported harness exit %d, want 2", code)
	}
	stdout.Reset()
	if code := runPolicyApply([]string{"--help"}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("help exit %d, want 0", code)
	}
}
