// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// hasFlag reports whether args holds flag immediately followed by value.
func hasFlag(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// A Claude worker must never load the settings files of the directory it runs
// in (hooks, env, permissions run there without a trust prompt under -p), and a
// read worker must hold only reading tools. These flags are pinned on the
// launch and the resume of both modes.
func TestClaudeWorkerArgvPinsIsolationFlags(t *testing.T) {
	read, _ := claudeReadOnly(Spec{})
	write, _ := claudeWrite(Spec{})
	cases := map[string]struct {
		args []string
		read bool
	}{
		"read launch":  {read.args, true},
		"read resume":  {claudeResumeArgs("plan", Spec{}, "11111111-2222-4333-8444-555555555555"), true},
		"write launch": {write.args, false},
		"write resume": {claudeResumeArgs("bypassPermissions", Spec{}, "11111111-2222-4333-8444-555555555555"), false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if !hasFlag(c.args, "--setting-sources", "user") || !slices.Contains(c.args, "--strict-mcp-config") {
				t.Errorf("isolation missing: %v", c.args)
			}
			readOnly := hasFlag(c.args, "--tools", "Read,Grep,Glob") && hasFlag(c.args, "--permission-prompts", "none")
			if readOnly != c.read {
				t.Errorf("read tool limit = %v, want %v: %v", readOnly, c.read, c.args)
			}
			for i, a := range c.args {
				if a == "--tools" && (i+2 >= len(c.args) || !strings.HasPrefix(c.args[i+2], "--")) {
					t.Errorf("--tools must be followed by another flag so it swallows no argument: %v", c.args)
				}
			}
		})
	}
}

// The worker process gets the filtered environment: what the CLI needs passes,
// an unrelated token does not, and the spec's own entries always pass.
func TestWorkerProcessGetsTheFilteredEnvironment(t *testing.T) {
	dir := shim(t, "claude", `env > "$(dirname "$0")/env.out"; echo '{"type":"result","subtype":"success","is_error":false,"result":"ok"}'`)
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute, Env: []string{"EXTRA_FROM_SPEC=1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(dir, "env.out"))
	for _, want := range []string{"ANTHROPIC_API_KEY=sk-test", "EXTRA_FROM_SPEC=1", WorkerMarkerEnv + "=1", "PATH="} {
		if !strings.Contains(got, want) {
			t.Errorf("worker env lacks %q", want)
		}
	}
	if strings.Contains(got, "gh-secret") {
		t.Error("an unrelated token reached the worker")
	}
}

func TestWorkerEnvFilter(t *testing.T) {
	base := []string{
		"PATH=/bin", "HOME=/h", "LC_ALL=C", "ANTHROPIC_API_KEY=k", "OPENAI_API_KEY=o", "CODEX_HOME=/c",
		"GITHUB_TOKEN=g", "GH_TOKEN=g", "NPM_TOKEN=n", "SSH_AUTH_SOCK=/s", "DATABASE_URL=d",
		"AWS_SECRET_ACCESS_KEY=a", "GOOGLE_APPLICATION_CREDENTIALS=/g", "Path=C:\\w",
	}
	cases := []struct {
		name       string
		extra      []string
		keep, drop []string
	}{
		{"base", nil,
			[]string{"PATH=/bin", "HOME=/h", "LC_ALL=C", "ANTHROPIC_API_KEY=k", "OPENAI_API_KEY=o", "CODEX_HOME=/c", "Path=C:\\w"},
			[]string{"GITHUB_TOKEN=g", "GH_TOKEN=g", "NPM_TOKEN=n", "SSH_AUTH_SOCK=/s", "DATABASE_URL=d", "AWS_SECRET_ACCESS_KEY=a", "GOOGLE_APPLICATION_CREDENTIALS=/g"}},
		{"bedrock passes AWS", []string{"CLAUDE_CODE_USE_BEDROCK=1"},
			[]string{"AWS_SECRET_ACCESS_KEY=a"}, []string{"GOOGLE_APPLICATION_CREDENTIALS=/g"}},
		{"vertex passes Google", []string{"CLAUDE_CODE_USE_VERTEX=1"},
			[]string{"GOOGLE_APPLICATION_CREDENTIALS=/g"}, []string{"AWS_SECRET_ACCESS_KEY=a"}},
		{"empty switch passes nothing", []string{"CLAUDE_CODE_USE_BEDROCK="},
			nil, []string{"AWS_SECRET_ACCESS_KEY=a"}},
		{"extra names", []string{ExtraEnvNamesEnv + "=DATABASE_URL, npm_token"},
			[]string{"DATABASE_URL=d", "NPM_TOKEN=n"}, []string{"GITHUB_TOKEN=g"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := workerEnv(append(slices.Clone(base), c.extra...))
			for _, k := range c.keep {
				if !slices.Contains(got, k) {
					t.Errorf("dropped %q", k)
				}
			}
			for _, d := range c.drop {
				if slices.Contains(got, d) {
					t.Errorf("kept %q", d)
				}
			}
		})
	}
}
