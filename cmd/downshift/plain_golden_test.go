// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// TestPlainGolden pins the plain (stdio piped, not a terminal) stdout, stderr
// and exit code of the read-only console commands, so a change that routes
// human TTY runs into the TUI can prove machine output stays byte-for-byte
// identical. Regenerate with:
//
//	go test ./cmd/downshift -run TestPlainGolden -update
var updateGoldens = flag.Bool("update", false, "rewrite testdata/plain/*.golden")

// plainSkips lists read-only console commands that are not pinned, with why.
var plainSkips = map[string]string{
	"models available --harness": "needs a harness id argument",
	"models discover":            "probes harness CLIs",
	"models check":               "queries provider APIs over the network",
	"route --harness":            "needs a harness id argument",
	"quota status --harness":     "needs a harness id argument",
	"gateway status":             "probes local ports; depends on the machine",
	"compact --":                 "needs a command after --",
	"plan":                       "reads a plan from stdin",
	"doctor":                     "probes the machine; not deterministic",
	"doctor --verbose":           "probes the machine; not deterministic",
	"context benchmark":          "timing dependent",
	"context doctor":             "probes installed providers on the machine",
	"context metrics":            "asks a provider; depends on the machine",
	"context providers":          "provider order is map-iteration random today",
	"eval-outcome --report":      "needs labelled data on disk",
}

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// plainBinary builds the real binary once: most commands call os.Exit from
// main, so they cannot be driven in-process.
func plainBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir := t.TempDir() // removed with the calling test, so no stray binary is left behind
		binPath = filepath.Join(dir, "downshift")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", binPath, ".").CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return binPath
}

// plainEnv is a minimal environment: temp HOME, an empty PATH directory (no
// harness CLI is found), and no DOWNSHIFT_* variable (DOWNSHIFT_PLAIN unset).
func plainEnv(home, emptyPath string) []string {
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + emptyPath, "NO_COLOR=1"}
	for _, k := range []string{"SystemRoot", "TMPDIR", "TEMP", "TMP"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// Volatile parts replaced before comparing.
var (
	reVersion  = regexp.MustCompile(`\bv?\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?\b`)
	reStamp    = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?`)
	reDuration = regexp.MustCompile(`\b\d+(\.\d+)?(ns|µs|ms|s)\b`)
	// The Claude desktop config dir in the "looked for" list differs per OS.
	reDesktopDir = regexp.MustCompile(`, ~/[^,)]*Claude\)`)
)

// normalisePlain makes output comparable across machines and OSes: CRLF to LF,
// the binary, temp HOME, PATH dir and temp root to placeholders, path separators to "/", and
// versions, timestamps and durations to tokens.
func normalisePlain(s, home, emptyPath string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for _, p := range []struct{ from, to string }{{binPath, "<BIN>"}, {home, "<HOME>"}, {emptyPath, "<EMPTYPATH>"}, {os.TempDir(), "<TMP>"}} {
		s = strings.ReplaceAll(s, p.from, p.to)
		s = strings.ReplaceAll(s, filepath.ToSlash(p.from), p.to)
	}
	s = strings.ReplaceAll(s, `\`, "/")
	s = reDesktopDir.ReplaceAllString(s, ")")
	s = reStamp.ReplaceAllString(s, "<TIME>")
	s = reVersion.ReplaceAllString(s, "<VERSION>")
	return reDuration.ReplaceAllString(s, "<DUR>")
}

func goldenName(args []string) string {
	n := strings.Join(args, "_")
	return strings.Trim(strings.NewReplacer("-", "", "=", "_").Replace(n), "_") + ".golden"
}

func TestPlainGolden(t *testing.T) {
	bin := plainBinary(t)
	for _, spec := range consoleCommands {
		if spec.Writes || spec.Terminal {
			continue
		}
		key := strings.Join(spec.Args, " ")
		t.Run(key, func(t *testing.T) {
			if why, skip := plainSkips[key]; skip {
				t.Skip(why)
			}
			home, emptyPath := t.TempDir(), t.TempDir()
			cmd := exec.Command(bin, spec.Args...)
			cmd.Env = plainEnv(home, emptyPath)
			cmd.Dir = home
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			code := 0
			if err := cmd.Run(); err != nil {
				ee, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				code = ee.ExitCode()
			}
			got := fmt.Sprintf("exit: %d\n--- stdout ---\n%s--- stderr ---\n%s",
				code, normalisePlain(out.String(), home, emptyPath), normalisePlain(errOut.String(), home, emptyPath))
			path := filepath.Join("testdata", "plain", goldenName(spec.Args))
			if *updateGoldens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s (run with -update): %v", path, err)
			}
			if string(want) != got {
				t.Errorf("plain output of `downshift %s` changed\n--- want ---\n%s\n--- got ---\n%s", key, want, got)
			}
		})
	}
}
