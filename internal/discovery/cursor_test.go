// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Shape verified against Cursor CLI's distributed formatter (2026.09.10).
// IDs are synthetic: discovery must never depend on a fixed product model list.
const cursorFixture = "Available models\n\nmodel-tiny - Tiny (current, default)\nmodel-large[effort=high] - Large\nmodel-bare\n\nTip: use --model <id> (or /model <id> in interactive mode) to switch.\n"

func TestParseCursorList(t *testing.T) {
	got, err := parseCursorList([]byte(cursorFixture))
	want := []Model{{ID: "model-tiny", Display: "Tiny"}, {ID: "model-large[effort=high]", Display: "Large"}, {ID: "model-bare"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
	ansi := strings.ReplaceAll(cursorFixture, "Available models", "\x1b[2mAvailable models\x1b[0m")
	ansi = strings.ReplaceAll(ansi, "model-tiny", "\x1b[32mmodel-tiny\x1b[0m")
	if got, err = parseCursorList([]byte(ansi)); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ANSI got %+v, %v", got, err)
	}
}

func TestParseCursorRejectsUnknownOrPartialOutput(t *testing.T) {
	for name, data := range map[string]string{
		"empty":            "",
		"unauthenticated":  "Error: Authentication required.",
		"no models":        "No models available for this account.",
		"incomplete":       "Available models\nmodel-tiny - Tiny\n",
		"empty block":      "Available models\nTip: use --model <id>\n",
		"malformed":        "Available models\nnot a model row\nTip: use --model <id>\n",
		"duplicate":        "Available models\nmodel-tiny - Tiny\nmodel-tiny - Tiny\nTip: use --model <id>\n",
		"trailing warning": cursorFixture + "Warning: request failed\n",
		"scanner overflow": "Available models\n" + strings.Repeat("a", 128*1024) + "\nTip: use --model <id>\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := parseCursorList([]byte(data)); err == nil {
				t.Fatalf("accepted %+v", got)
			}
		})
	}
}

func cursorScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCursorCLIUsesOnlyModelsCommand(t *testing.T) {
	p := cursorScript(t, t.TempDir(), "cursor-test", "[ \"$#\" = 1 ] && [ \"$1\" = models ] || exit 9\ncat <<'MODELS'\n"+cursorFixture+"MODELS\n")
	got, err := (CursorCLI{Bin: p}).Discover(context.Background())
	if err != nil || len(got) != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestCursorCLIFailuresAreUnknown(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"auth":      "exit 1\n",
		"malformed": "echo incorrect\n",
		"empty":     "exit 0\n",
		"timeout":   "while :; do :; done\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := cursorScript(t, dir, name, body)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			got, err := (CursorCLI{Bin: p}).Discover(ctx)
			if err == nil || len(got) != 0 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestCursorCLIVerifiesGenericAgent(t *testing.T) {
	for _, cursor := range []bool{false, true} {
		t.Run(map[bool]string{false: "unrelated", true: "cursor"}[cursor], func(t *testing.T) {
			dir := t.TempDir()
			body := "echo unrelated agent\n"
			if cursor {
				body = "if [ \"$2\" = --help ]; then\n echo 'Usage: agent models [options]'; echo 'List available models for this account'; exit 0\nfi\n[ \"$1\" = models ] || exit 9\nprintf '%s\\n' 'Available models' 'model-tiny - Tiny' 'Tip: use --model <id>'\n"
			}
			cursorScript(t, dir, "agent", body)
			t.Setenv("PATH", dir)
			got, err := (CursorCLI{}).Discover(context.Background())
			if cursor && (err != nil || len(got) != 1) {
				t.Fatalf("got %+v, %v", got, err)
			}
			if !cursor && err == nil {
				t.Fatalf("unrelated agent accepted: %+v", got)
			}
		})
	}
}
