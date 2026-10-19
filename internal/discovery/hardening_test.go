// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCLIEnvKeepsOnlyTheAllowlistAndTheCLIsOwnPrefix(t *testing.T) {
	environ := []string{
		"PATH=/usr/bin", "HOME=/h", "LANG=C",
		"ANTHROPIC_API_KEY=a", "OPENAI_API_KEY=o", "XAI_API_KEY=x", "GITHUB_TOKEN=g", "AWS_SECRET_ACCESS_KEY=s",
		"KIRO_PROFILE=p", "CURSOR_CONFIG=c", "malformed",
	}
	has := func(env []string, name string) bool {
		for _, kv := range env {
			if strings.HasPrefix(kv, name+"=") {
				return true
			}
		}
		return false
	}
	for _, bin := range []string{"grok", "kiro-cli", "cursor-agent", "/usr/local/bin/agent"} {
		env := cliEnv(environ, bin)
		for _, secret := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY"} {
			if has(env, secret) {
				t.Errorf("%s: %s leaked into the CLI environment", bin, secret)
			}
		}
		if !has(env, "PATH") || !has(env, "HOME") || !has(env, "LANG") {
			t.Errorf("%s: basics missing: %v", bin, env)
		}
	}
	if !has(cliEnv(environ, "kiro-cli"), "KIRO_PROFILE") || has(cliEnv(environ, "kiro-cli"), "CURSOR_CONFIG") {
		t.Error("kiro-cli keeps KIRO_ only")
	}
	if !has(cliEnv(environ, "cursor-agent"), "CURSOR_CONFIG") || has(cliEnv(environ, "grok"), "KIRO_PROFILE") {
		t.Error("prefixes are per CLI")
	}
}

// End to end: a real child process must not see the provider keys.
func TestRunCLIChildDoesNotInheritProviderKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fakecli")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nenv\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-not-leak")
	t.Setenv("OPENAI_API_KEY", "sk-should-not-leak")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runCLI(ctx, script)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "should-not-leak") || strings.Contains(string(out), "API_KEY") {
		t.Fatalf("provider keys reached the child:\n%s", out)
	}
	if !strings.Contains(string(out), "PATH=") {
		t.Errorf("the child lost PATH:\n%s", out)
	}
}

func TestRunValidatesIDsFromEverySource(t *testing.T) {
	src := fakeSource{models: []Model{
		{ID: "good-1"}, {ID: "evil\x1b[2J"}, {ID: "has space"}, {ID: ""}, {ID: strings.Repeat("a", 200)}, {ID: "ok.v2:beta"},
	}}
	out := Run(context.Background(), []Source{src}, time.Second)
	var ids []string
	for _, m := range out[0].Models {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "good-1,ok.v2:beta" {
		t.Fatalf("ids = %v", ids)
	}
	allBad := Run(context.Background(), []Source{fakeSource{models: []Model{{ID: "x y"}}}}, time.Second)
	if allBad[0].Err == nil {
		t.Error("a source whose every id is invalid must be an error, not an empty success")
	}
}

type fakeSource struct{ models []Model }

func (fakeSource) Harness() string { return "h" }
func (fakeSource) Name() string    { return "fake" }
func (f fakeSource) Discover(context.Context) ([]Model, error) {
	return f.models, nil
}
