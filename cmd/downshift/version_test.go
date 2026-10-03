// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestReleaseLdflagsTargetExistingVars guards against -X flags in
// .goreleaser.yml pointing at variables that do not exist: the linker ignores
// them silently and every release reports binary_version "dev".
func TestReleaseLdflagsTargetExistingVars(t *testing.T) {
	cfg, err := os.ReadFile("../../.goreleaser.yml")
	if err != nil {
		t.Fatal(err)
	}
	targets := regexp.MustCompile(`-X main\.(\w+)=`).FindAllStringSubmatch(string(cfg), -1)
	if len(targets) == 0 {
		t.Fatal("no -X main.<var> ldflags found in .goreleaser.yml")
	}

	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]bool{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				vars[name.Name] = true
			}
		}
	}
	for _, m := range targets {
		if !vars[m[1]] {
			t.Errorf(".goreleaser.yml sets main.%s, but cmd/downshift/main.go declares no such var", m[1])
		}
	}
	if !strings.Contains(string(cfg), "-X main.buildVersion=") {
		t.Error(".goreleaser.yml must stamp main.buildVersion (used in telemetry binary_version)")
	}
}

func TestRunVersion(t *testing.T) {
	oldV, oldC, oldD := buildVersion, buildCommit, buildDate
	defer func() { buildVersion, buildCommit, buildDate = oldV, oldC, oldD }()

	var buf bytes.Buffer
	buildVersion, buildCommit, buildDate = "dev", "", ""
	if code := runVersion(&buf); code != 0 || buf.String() != "downshift dev\n" {
		t.Errorf("dev build: code=%d out=%q", code, buf.String())
	}

	buf.Reset()
	buildVersion, buildCommit, buildDate = "1.2.3", "abc1234", "2026-10-03T00:00:00Z"
	runVersion(&buf)
	if want := "downshift 1.2.3 (commit abc1234, built 2026-10-03T00:00:00Z)\n"; buf.String() != want {
		t.Errorf("release build: got %q, want %q", buf.String(), want)
	}
}
