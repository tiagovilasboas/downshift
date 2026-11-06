// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runPlanT(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	playbookEnv(t)
	var out, errb bytes.Buffer
	code := runPlan(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

const planOK = `{"goal":"g","parts":[{"id":"a","title":"A","agent":"coder","points":3},{"id":"b","agent":"tester","points":2,"depends_on":["a"]}]}`

func TestPlanMarkdownAndJSON(t *testing.T) {
	code, out, e := runPlanT(t, planOK)
	if code != 0 || !strings.Contains(out, "| b | b | tester | ") || !strings.Contains(out, "Total: 5 points in 2 waves.") {
		t.Fatalf("code %d out %q err %q", code, out, e)
	}
	code, out, _ = runPlanT(t, planOK, "--json")
	if code != 0 || !strings.Contains(out, `"wave": 2`) {
		t.Errorf("json code %d out %q", code, out)
	}
}

func TestPlanInvalidExitsOneWithAllErrors(t *testing.T) {
	code, _, e := runPlanT(t, `{"parts":[{"id":"a","agent":"x","points":8},{"id":"a","agent":"coder","points":1}]}`)
	if code != 1 || !strings.Contains(e, "unknown agent") || !strings.Contains(e, "split this part") || !strings.Contains(e, "duplicate") {
		t.Errorf("code %d err %q", code, e)
	}
	if code, _, _ := runPlanT(t, `nope`); code != 1 {
		t.Errorf("bad json code %d", code)
	}
}

func TestPlanRouteFollowsGatewaySwitch(t *testing.T) {
	const worker = `{"goal":"g","parts":[{"id":"w","title":"W","agent":"coder","points":1,"worker":"claude"}]}`
	dir := playbookEnv(t)
	t.Setenv("DOWNSHIFT_WORKERS", "bypass")
	var out, errb bytes.Buffer
	if code := runPlan(nil, strings.NewReader(worker), &out, &errb); code != 0 || !strings.Contains(out.String(), "| direct |") {
		t.Fatalf("off: code %d out %q err %q", code, out.String(), errb.String())
	}
	if err := os.WriteFile(filepath.Join(dir, "gateway.json"), []byte(`{"enabled":true,"name":"9router"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := runPlan([]string{"--json"}, strings.NewReader(worker), &out, &errb); code != 0 ||
		!strings.Contains(out.String(), `"route": "9router"`) || !strings.Contains(out.String(), `"kind": "delegate"`) {
		t.Errorf("on: code %d out %q err %q", code, out.String(), errb.String())
	}
}

func TestPlanWorkersByMode(t *testing.T) {
	const worker = `{"goal":"g","parts":[{"id":"w","title":"W","agent":"coder","points":1,"worker":"claude"}]}`
	defWarn := "needs write workers" // the default is read for claude
	if runtime.GOOS == "windows" {
		defWarn = "needs workers on" // and off on Windows
	}
	cases := []struct {
		env, kind, mode, warn string
	}{
		{"", "spawn", "", defWarn},
		{"off", "spawn", "", "needs workers on"},
		{"read", "spawn", "", "needs write workers"},
		{"bypass", "delegate", "write", ""},
	}
	for _, c := range cases {
		t.Run("env="+c.env, func(t *testing.T) {
			playbookEnv(t)
			t.Setenv("DOWNSHIFT_WORKERS", c.env)
			var out, errb bytes.Buffer
			if code := runPlan([]string{"--json"}, strings.NewReader(worker), &out, &errb); code != 0 {
				t.Fatalf("code %d err %q", code, errb.String())
			}
			got := out.String()
			if !strings.Contains(got, `"kind": "`+c.kind+`"`) || (c.mode != "" && !strings.Contains(got, `"mode": "`+c.mode+`"`)) {
				t.Errorf("out %s", got)
			}
			if c.warn != "" && !strings.Contains(got, c.warn) {
				t.Errorf("no warning %q in %s", c.warn, got)
			}
			if c.kind == "spawn" && strings.Contains(got, `"kind": "delegate"`) {
				t.Errorf("delegate with workers %q", c.env)
			}
		})
	}
}
