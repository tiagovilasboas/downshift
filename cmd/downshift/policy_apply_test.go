// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func setupPolicyApplyEnv(t *testing.T, policyJSON string) (dir, home string) {
	t.Helper()
	dir = t.TempDir()
	home = t.TempDir()
	pDir := filepath.Join(dir, ".downshift")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, home
}

func TestPolicyApply_CompilationCreatesNewAgent(t *testing.T) {
	policyJSON := `{
  "version": 1,
  "profiles": [
    {"name": "researcher", "tier": "small", "harness": "claude-code"},
    {"name": "coder", "tier": "mid"}
  ]
}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	var out, errOut bytes.Buffer
	opts := policyApplyOptions{
		Harness: "claude-code",
		Yes:     true,
		Dir:     dir,
		Home:    home,
		Out:     &out,
		Err:     &errOut,
		Now:     time.Now,
	}

	code := executePolicyApply(opts)
	if code != 0 {
		t.Fatalf("executePolicyApply exit %d; stderr: %s", code, errOut.String())
	}

	cat := catalog.Load()
	smallModel := cat.ModelFor("claude-code", core.TierSmall).ID
	midModel := cat.ModelFor("claude-code", core.TierMid).ID

	resPath := filepath.Join(dir, ".claude", "agents", "researcher.md")
	raw, err := os.ReadFile(resPath)
	if err != nil {
		t.Fatalf("ReadFile researcher.md: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "name: researcher") || !strings.Contains(body, "model: "+smallModel) {
		t.Errorf("researcher.md missing expected frontmatter:\n%s", body)
	}

	coderPath := filepath.Join(dir, ".claude", "agents", "coder.md")
	rawCoder, err := os.ReadFile(coderPath)
	if err != nil {
		t.Fatalf("ReadFile coder.md: %v", err)
	}
	if !strings.Contains(string(rawCoder), "model: "+midModel) {
		t.Errorf("coder.md missing midModel:\n%s", string(rawCoder))
	}
}

func TestPolicyApply_DryRunDoesNotTouchFiles(t *testing.T) {
	policyJSON := `{"version": 1, "profiles": [{"name": "scout", "tier": "small"}]}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	var out, errOut bytes.Buffer
	opts := policyApplyOptions{
		Harness: "claude-code",
		DryRun:  true,
		Dir:     dir,
		Home:    home,
		Out:     &out,
		Err:     &errOut,
	}

	code := executePolicyApply(opts)
	if code != 0 {
		t.Fatalf("dry run exit %d: %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "scout.md")); err == nil {
		t.Error("file created during dry run")
	}
	if !strings.Contains(out.String(), "Dry run: nothing was written") {
		t.Errorf("dry run notice missing:\n%s", out.String())
	}
}

func TestPolicyApply_UpdatesExistingAgentPreservingBodyAndKeys(t *testing.T) {
	policyJSON := `{"version": 1, "profiles": [{"name": "tester", "tier": "small"}]}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	agentDir := filepath.Join(dir, ".claude", "agents")
	_ = os.MkdirAll(agentDir, 0o755)
	existing := `---
name: tester
model: claude-opus-old
tools: [Bash, Read]
---

# Test instructions
Execute unit tests thoroughly.
`
	filePath := filepath.Join(agentDir, "tester.md")
	_ = os.WriteFile(filePath, []byte(existing), 0o644)

	var out, errOut bytes.Buffer
	opts := policyApplyOptions{
		Harness: "claude-code",
		Yes:     true,
		Dir:     dir,
		Home:    home,
		Out:     &out,
		Err:     &errOut,
		Now:     time.Now,
	}

	if code := executePolicyApply(opts); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	smallModel := catalog.Load().ModelFor("claude-code", core.TierSmall).ID
	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	uText := string(updated)
	if !strings.Contains(uText, "model: "+smallModel) {
		t.Errorf("expected model %s, got:\n%s", smallModel, uText)
	}
	if !strings.Contains(uText, "tools: [Bash, Read]") {
		t.Errorf("tools field lost:\n%s", uText)
	}
	if !strings.Contains(uText, "Execute unit tests thoroughly.") {
		t.Errorf("body text lost:\n%s", uText)
	}
}

func TestPolicyApply_Idempotent(t *testing.T) {
	policyJSON := `{"version": 1, "profiles": [{"name": "worker", "tier": "small"}]}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	var out bytes.Buffer
	opts := policyApplyOptions{
		Harness: "claude-code",
		Yes:     true,
		Dir:     dir,
		Home:    home,
		Out:     &out,
		Now:     time.Now,
	}

	if code := executePolicyApply(opts); code != 0 {
		t.Fatalf("first apply exit %d", code)
	}

	out.Reset()
	if code := executePolicyApply(opts); code != 0 {
		t.Fatalf("second apply exit %d", code)
	}
	if !strings.Contains(out.String(), "already up to date") {
		t.Errorf("second apply not reported as up to date:\n%s", out.String())
	}
}

func TestPolicyApply_CreatesBackupBeforeModification(t *testing.T) {
	policyJSON := `{"version": 1, "profiles": [{"name": "writer", "tier": "mid"}]}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	agentDir := filepath.Join(dir, ".claude", "agents")
	_ = os.MkdirAll(agentDir, 0o755)
	orig := "---\nname: writer\nmodel: old\n---\nBody\n"
	agentFile := filepath.Join(agentDir, "writer.md")
	_ = os.WriteFile(agentFile, []byte(orig), 0o644)

	opts := policyApplyOptions{
		Harness: "claude-code",
		Yes:     true,
		Dir:     dir,
		Home:    home,
		Now:     time.Now,
	}
	if code := executePolicyApply(opts); code != 0 {
		t.Fatalf("apply exit %d", code)
	}

	matches, _ := filepath.Glob(filepath.Join(agentDir, "writer.md.downshift-bak-*"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 backup, found %d", len(matches))
	}
	bakData, _ := os.ReadFile(matches[0])
	if string(bakData) != orig {
		t.Errorf("backup content mismatch: got %q, want %q", string(bakData), orig)
	}
}

func TestPolicyApply_RejectsPathTraversalProfiles(t *testing.T) {
	policyJSON := `{
  "version": 1,
  "profiles": [
    {"name": "../evil", "tier": "small"},
    {"name": "sub/agent", "tier": "small"},
    {"name": "valid-agent", "tier": "small"}
  ]
}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	var errOut bytes.Buffer
	opts := policyApplyOptions{
		Harness: "claude-code",
		Yes:     true,
		Dir:     dir,
		Home:    home,
		Err:     &errOut,
		Now:     time.Now,
	}

	if code := executePolicyApply(opts); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "skipping profile") {
		t.Errorf("expected warning for traversal names, got:\n%s", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "agents", "valid-agent.md")); err != nil {
		t.Errorf("valid agent was not created: %v", err)
	}
}

func TestPolicyApply_ResolvesSymlinks(t *testing.T) {
	policyJSON := `{"version": 1, "profiles": [{"name": "symagent", "tier": "small"}]}`
	dir, home := setupPolicyApplyEnv(t, policyJSON)
	realDir := filepath.Join(dir, "shared_agents")
	agentDir := filepath.Join(dir, ".claude", "agents")
	_ = os.MkdirAll(realDir, 0o755)
	_ = os.MkdirAll(agentDir, 0o755)

	realFile := filepath.Join(realDir, "symagent.md")
	_ = os.WriteFile(realFile, []byte("---\nname: symagent\nmodel: old\n---\nPrompt"), 0o644)

	symlinkFile := filepath.Join(agentDir, "symagent.md")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	opts := policyApplyOptions{
		Harness: "claude-code",
		Yes:     true,
		Dir:     dir,
		Home:    home,
		Now:     time.Now,
	}
	if code := executePolicyApply(opts); code != 0 {
		t.Fatalf("exit %d", code)
	}

	fi, err := os.Lstat(symlinkFile)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink was destroyed: %v", err)
	}
	raw, _ := os.ReadFile(realFile)
	smallModel := catalog.Load().ModelFor("claude-code", core.TierSmall).ID
	if !strings.Contains(string(raw), "model: "+smallModel) {
		t.Errorf("target of symlink not updated:\n%s", string(raw))
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
